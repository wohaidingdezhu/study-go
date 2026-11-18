"""用独立 Docker 项目验证 Spring Boot 和真实 MySQL 的订单契约。"""

import argparse
import json
import os
from pathlib import Path
import subprocess
from urllib.error import HTTPError
from urllib.request import ProxyHandler, Request, build_opener
from uuid import uuid4


COMPOSE_FILE = Path(__file__).resolve().with_name("compose.yaml")
ROOT = COMPOSE_FILE.parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reuse-image", action="store_true", help="使用已构建的 Spring Boot 镜像")
    args = parser.parse_args()
    project = "study-go-spring-check-" + uuid4().hex[:8]
    env = {**os.environ, "SPRING_ORDERS_PORT": "0", "NO_PROXY": "127.0.0.1,localhost", "no_proxy": "127.0.0.1,localhost"}

    def compose(*args, timeout=25):
        command = ["docker", "compose", "-p", project, "-f", str(COMPOSE_FILE), *args]
        result = subprocess.run(command, env=env, capture_output=True, text=True, timeout=timeout)
        if result.returncode:
            raise RuntimeError(f"Docker {args[0]} 失败\n{result.stdout}{result.stderr}")
        return result.stdout.strip()

    def sql(statement):
        return compose("exec", "-T", "mysql", "sh", "-ec",
                       'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot -Dorders -Nse "$1"', "sh", statement)

    try:
        compose("up", "--no-build" if args.reuse_image else "--build", "--scale", "api=2",
                "--wait", "--wait-timeout", "100", timeout=300)
        addresses = [compose("port", "--index", str(index), "api", "8080") for index in (1, 2)]
        assert len(set(addresses)) == 2, addresses
        assert all(address.startswith("127.0.0.1:") and not address.endswith(":0") for address in addresses), addresses
        base, peer = (f"http://{address}" for address in addresses)
        opener = build_opener(ProxyHandler({}))

        def request(path, *, data=None, key=None, content_type="application/json", target=base):
            headers = {"Content-Type": content_type, "Idempotency-Key": key} if key else {}
            request = Request(target + path, data=data, headers=headers)
            try:
                response = opener.open(request, timeout=8)
            except HTTPError as error:
                response = error
            with response:
                return response.status, json.load(response)

        assert request("/orders", data=b'{"productId":1,"quantity":"1"}', key="invalid") == (400, {"error": "invalid_order"})
        assert request("/orders", data=b'{}', key="invalid-type", content_type="text/plain") == (415, {"error": "expected_json"})
        # 在扣库存之后故意让订单 INSERT 失败，确认库存、订单和幂等键一起回滚。
        sql("CREATE TRIGGER check_reject_insert BEFORE INSERT ON orders FOR EACH ROW "
            "SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced rollback'")
        try:
            failure = request("/orders", data=b'{"productId":1,"quantity":1}', key="rollback-check")
            assert failure == (500, {"error": "internal_error"}), failure
            assert sql("SELECT stock, (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM order_keys) FROM products WHERE id=1") == "5\t0\t0"
        finally:
            sql("DROP TRIGGER check_reject_insert")
        print("通过：拒绝无效输入，强制写订单失败时库存与幂等键一起回滚。", flush=True)

        subprocess.run(["python3", str(ROOT / "scripts/check-orders.py"), base, "--peer-url", peer], env=env, check=True, timeout=80)
        assert sql("SELECT COUNT(*), COUNT(order_id) FROM order_keys") == "4\t4"
        compose("stop", "mysql")
        for target in (base, peer):
            assert request("/livez", target=target) == (200, {"status": "live"})
            assert request("/readyz", target=target) == (503, {"status": "not_ready"})
        print("通过：MySQL 停机时，两个 Java 进程仍存活，就绪探针均返回 503。", flush=True)
    finally:
        # 仅清理这次随机项目的资源。手工练习的数据库卷保留。
        compose("down", "--volumes", "--remove-orphans", timeout=40)


if __name__ == "__main__":
    main()
