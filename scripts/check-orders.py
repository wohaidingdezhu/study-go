"""对一套刚初始化的 Java 或 Go 服务运行同一组下单练习。"""

import argparse
import json
import re
import subprocess
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def check_orders(base_url, peer_url=None):
    prefix = uuid.uuid4().hex

    def call(path, key=None, quantity=1, product_id=1, request_id=None, *, target=base_url):
        request = Request(target.rstrip("/") + path)
        if request_id is not None:
            request.add_header("X-Request-ID", request_id)
        if key is not None:
            request.method = "POST"
            request.data = json.dumps({"productId": product_id, "quantity": quantity}).encode()
            request.add_header("Content-Type", "application/json")
            request.add_header("Idempotency-Key", f"{prefix}-{key}")
        try:
            response = urlopen(request, timeout=10)
        except HTTPError as error:
            response = error
        with response:
            response_id = response.headers.get("X-Request-ID", "")
            assert re.fullmatch(r"[a-zA-Z0-9_-]{1,64}", response_id), (path, response_id)
            if request_id is not None and re.fullmatch(r"[a-zA-Z0-9_-]{1,64}", request_id):
                assert response_id == request_id, (path, response_id)
            return response.code, json.load(response)

    # 检查服务生成的 ID、合法的来访 ID 和无效 ID 被替换后的响应。
    assert call("/livez") == (200, {"status": "live"})
    assert call("/readyz", request_id="invalid/lesson-id") == (200, {"status": "ready"})
    initial_product = {"id": 1, "name": "Notebook", "stock": 5}
    empty_page = {"items": [], "nextCursor": None}
    # 并发用例会消耗库存，两个地址都应指向一套全新的练习库。
    for target in (base_url, peer_url) if peer_url else (base_url,):
        assert call("/products/1", target=target) == (200, initial_product), "Start a fresh service with stock 5."
        assert call("/orders", target=target) == (200, empty_page), "Start with an empty order list."
    assert call("/products/999", request_id="study-not-found") == (404, {"error": "not_found"})
    assert call("/orders", "invalid", 0) == (400, {"error": "invalid_order"})
    assert call("/orders", "missing", product_id=999) == (404, {"error": "not_found"})
    assert call("/orders", "retry", 6) == (409, {"error": "out_of_stock"})

    def target_for(index):
        return peer_url if peer_url and index % 2 else base_url

    # 相同幂等键并发发出 8 次，预期只有第一次写入会扣库存。
    # 提供第二个地址时交替发往两个 API，验证数据库约束能跨进程生效。
    with ThreadPoolExecutor(max_workers=8) as pool:
        replay = list(pool.map(lambda index: call("/orders", "retry", 2, target=target_for(index)), range(8)))

    assert sorted(status for status, _ in replay) == [200] * 7 + [201], replay
    order = replay[0][1]
    assert all(value == order for _, value in replay), replay
    assert call(f"/orders/{order['id']}") == (200, order)
    assert call("/orders", "retry", 1) == (409, {"error": "idempotency_conflict"})
    assert call("/products/1")[1]["stock"] == 3
    print("Same key × 8: one order, seven replays, stock 5 → 3.")

    # 改用 8 个不同的键，模拟不同订单争抢剩余库存。
    with ThreadPoolExecutor(max_workers=8) as pool:
        distinct = list(pool.map(lambda i: call("/orders", f"new-{i}", target=target_for(i)), range(8)))

    assert sorted(status for status, _ in distinct) == [201] * 3 + [409] * 5, distinct
    assert all(value == {"error": "out_of_stock"} for status, value in distinct if status == 409)
    assert call("/products/1")[1]["stock"] == 0
    print("Different keys × 8: three orders, five sold out, stock 3 → 0.")

    orders = [order, *(value for status, value in distinct if status == 201)]
    orders.sort(key=lambda value: value["id"], reverse=True)
    cursor = orders[1]["id"]
    # 四笔订单正好两页，第二页虽然也返回两条，游标应是 null。
    assert call("/orders?limit=%32") == (200, {"items": orders[:2], "nextCursor": cursor})
    assert call(f"/orders?limit=2&before={cursor}") == (
        200, {"items": orders[2:], "nextCursor": None}
    )
    assert call("/orders") == (200, {"items": orders, "nextCursor": None})
    assert call(f"/orders?before={orders[-1]['id']}") == (200, empty_page)
    if peer_url:
        assert call("/products/1", target=peer_url) == (200, {"id": 1, "name": "Notebook", "stock": 0})
        assert call("/orders", target=peer_url) == (200, {"items": orders, "nextCursor": None})
        print("两个 API 实例返回相同的订单与最终库存。")
    for query in ("limit=0", "limit=101", "limit=", "limit=1&limit=2",
                  "before=0", "before=-1", "before=nope", "before=9223372036854775808",
                  "page=1"):
        assert call(f"/orders?{query}") == (400, {"error": "invalid_pagination"}), query
    print("订单分页检查通过：两页不重不漏，空页与非法游标符合约定。")
    return [value["id"] for value in orders]


def check_practice_stack():
    stack_script = Path(__file__).with_name("practice-stack.sh").resolve()

    def stack(*args):
        return subprocess.check_output(["bash", str(stack_script), *args], text=True, timeout=10).strip()

    def query(database, sql):
        return stack("exec", "-T", "postgres", "psql", "-U", "study",
                     "-d", database, "-Atqc", sql)

    # 这组用例会用完初始库存。先确认这套练习里没有之前的学习数据。
    if (query("orders", "SELECT count(*) FROM orders") != "0"
            or query("notifications", "SELECT count(*) FROM order_notifications") != "0"):
        raise SystemExit("练习库已有记录，请保存需要的学习数据后再用新练习库运行。")

    address = stack("port", "api", "8082")
    order_ids = check_orders(f"http://{address}")
    ids = ", ".join(str(int(order_id)) for order_id in order_ids)
    deadline = time.monotonic() + 30
    published = received = ""
    while time.monotonic() < deadline:
        # 用同一组订单 ID 对照 outbox 和通知记录，等待异步消费完成。
        published = query("orders", "SELECT order_id, id FROM outbox "
                          f"WHERE order_id IN ({ids}) AND published_at IS NOT NULL ORDER BY order_id")
        received = query("notifications", "SELECT order_id, event_id FROM order_notifications "
                         f"WHERE order_id IN ({ids}) ORDER BY order_id")
        if len(published.splitlines()) == len(order_ids) and received == published:
            print(f"跨服务检查通过，共 {len(order_ids)} 笔订单的发布与通知记录匹配。")
            return
        time.sleep(0.5)

    raise SystemExit(
        "等待通知超时。可以查看 bash scripts/practice-stack.sh logs --tail=30 relay notifications。"
        f"\noutbox (order_id|event_id):\n{published}"
        f"\nnotifications (order_id|event_id):\n{received}"
    )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("base_url", nargs="?", help="A fresh service, e.g. http://127.0.0.1:8080")
    target.add_argument("--practice-stack", action="store_true", help="检查独立 Docker 练习的订单和通知")
    parser.add_argument("--peer-url", help="使用同一套新数据库的第二个 API 地址")
    args = parser.parse_args()
    if args.practice_stack:
        if args.peer_url:
            parser.error("--peer-url 需要指定主 API 地址")
        check_practice_stack()
    else:
        check_orders(args.base_url, args.peer_url)
