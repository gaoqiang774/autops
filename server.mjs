import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const CLIENT_DIR = path.join(__dirname, "dist", "client");
const SERVER_ENTRY = path.join(__dirname, "dist", "server", "index.js");

const PORT = parseInt(process.env.PORT || "3000", 10);
const HOST = process.env.HOST || "0.0.0.0";

const mimeTypes = {
  ".html": "text/html; charset=utf-8",
  ".js": "application/javascript; charset=utf-8",
  ".mjs": "application/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".txt": "text/plain; charset=utf-8"
};

// 预先动态加载打包好的 SSR Worker
let worker = null;
try {
  const workerModule = await import(pathToFileURL(SERVER_ENTRY).href);
  worker = workerModule.default;
  console.log("✅ AutoOps SSR Worker loaded successfully.");
} catch (err) {
  console.error("❌ Failed to load dist/server/index.js:", err);
}

function serveStaticFile(filePath, res) {
  const ext = path.extname(filePath).toLowerCase();
  const contentType = mimeTypes[ext] || "application/octet-stream";
  try {
    const stat = fs.statSync(filePath);
    if (!stat.isFile()) return false;
    res.writeHead(200, {
      "Content-Type": contentType,
      "Content-Length": stat.size,
      "Cache-Control": ext.match(/\.(js|css|woff2|png|jpg|svg)$/) ? "public, max-age=31536000, immutable" : "no-cache"
    });
    fs.createReadStream(filePath).pipe(res);
    return true;
  } catch (e) {
    return false;
  }
}

const server = http.createServer(async (req, res) => {
  const urlObj = new URL(req.url, `http://${req.headers.host || "localhost"}`);
  const pathname = decodeURIComponent(urlObj.pathname);

  // 1. 尝试静态文件服务 (dist/client)
  if (pathname !== "/") {
    const sanitizedPath = path.normalize(pathname).replace(/^(\.\.[\/\\])+/, "");
    const staticFilePath = path.join(CLIENT_DIR, sanitizedPath);
    if (fs.existsSync(staticFilePath) && fs.statSync(staticFilePath).isFile()) {
      return serveStaticFile(staticFilePath, res);
    }
  }

  // 2. 服务端 SSR / API 路由处理
  if (worker && typeof worker.fetch === "function") {
    try {
      const fullUrl = `http://${req.headers.host || "localhost"}${req.url}`;
      
      // 收集请求体
      let bodyBuffer = null;
      if (req.method !== "GET" && req.method !== "HEAD") {
        const chunks = [];
        for await (const chunk of req) {
          chunks.push(chunk);
        }
        bodyBuffer = Buffer.concat(chunks);
      }

      // 构造 Fetch API Request
      const fetchHeaders = new Headers();
      for (const [k, v] of Object.entries(req.headers)) {
        if (v) {
          if (Array.isArray(v)) {
            v.forEach(val => fetchHeaders.append(k, val));
          } else {
            fetchHeaders.set(k, v);
          }
        }
      }

      const fetchReq = new Request(fullUrl, {
        method: req.method,
        headers: fetchHeaders,
        body: bodyBuffer
      });

      // 执行 Worker
      const response = await worker.fetch(fetchReq, {
        ASSETS: {
          fetch: async (assetReq) => {
            const assetUrl = new URL(assetReq.url);
            const localFile = path.join(CLIENT_DIR, assetUrl.pathname);
            if (fs.existsSync(localFile) && fs.statSync(localFile).isFile()) {
              const fileData = fs.readFileSync(localFile);
              const ext = path.extname(localFile).toLowerCase();
              return new Response(fileData, {
                status: 200,
                headers: { "Content-Type": mimeTypes[ext] || "application/octet-stream" }
              });
            }
            return new Response("Not found", { status: 404 });
          }
        }
      }, {
        waitUntil() {},
        passThroughOnException() {}
      });

      // 将 Fetch API Response 写回到 Node.js http.ServerResponse
      const resHeaders = {};
      response.headers.forEach((val, key) => {
        resHeaders[key] = val;
      });

      res.writeHead(response.status, resHeaders);
      const arrayBuf = await response.arrayBuffer();
      res.end(Buffer.from(arrayBuf));
      return;
    } catch (err) {
      console.error("SSR Worker fetch error:", err);
      res.writeHead(500, { "Content-Type": "text/plain; charset=utf-8" });
      res.end("Internal Server Error: " + err.message);
      return;
    }
  }

  // 3. 兜底回退：若 Worker 未就绪，尝试返回静态 index.html
  const fallbackIndex = path.join(CLIENT_DIR, "index.html");
  if (fs.existsSync(fallbackIndex)) {
    return serveStaticFile(fallbackIndex, res);
  }

  res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
  res.end("404 Not Found");
});

server.listen(PORT, HOST, () => {
  console.log(`🚀 AutoOps production server running at http://${HOST}:${PORT}`);
});
