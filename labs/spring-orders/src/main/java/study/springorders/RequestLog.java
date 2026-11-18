package study.springorders;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.UUID;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
class RequestLog extends OncePerRequestFilter {
    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        String incoming = request.getHeader("X-Request-ID");
        String id = incoming != null && OrderController.REQUEST_TOKEN.matcher(incoming).matches()
                ? incoming : UUID.randomUUID().toString();
        response.setHeader("X-Request-ID", id);
        long start = System.nanoTime();
        try {
            chain.doFilter(request, response);
        } finally {
            // 用同一 ID 关联客户端响应和服务端日志。单调时钟适合计算耗时。
            LoggerFactory.getLogger(RequestLog.class).info("request_id={} method={} path={} status={} elapsed_ms={}",
                    id, request.getMethod(), request.getRequestURI(), response.getStatus(), (System.nanoTime() - start) / 1_000_000);
        }
    }
}
