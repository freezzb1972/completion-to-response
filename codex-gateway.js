/**
 * Codex Gateway — 多后端路由
 *
 *   Codex CLI → :8082/gateway → :8083/converter (deepseek)
 *                              → :8084/converter (nvidia)
 *
 * 环境变量:
 *   MODEL_MAP  — {"gpt-5.5":{"b":"deepseek","m":"deepseek-v4-pro"}, ...}
 *   BACKENDS   — {"deepseek":"8083","nvidia":"8084"}
 *   BACKEND_KEYS — {"deepseek":"sk-...","nvidia":"nvapi-..."}
 */

const http = require("http");
const PORT = parseInt(process.env.PROXY_PORT || "8082", 10);
const HOST = process.env.HOST || "127.0.0.1";

let modelMap = {};   // codex_model → { b: backend_name, m: backend_model }
let backends = {};   // backend_name → port
let backendKeys = {};

try { modelMap = JSON.parse(process.env.MODEL_MAP || "{}"); } catch (e) { console.error("invalid MODEL_MAP"); }
try { backends = JSON.parse(process.env.BACKENDS || "{}"); } catch (e) { console.error("invalid BACKENDS"); }
try { backendKeys = JSON.parse(process.env.BACKEND_KEYS || "{}"); } catch (e) { console.error("invalid BACKEND_KEYS"); }

const codexModels = Object.keys(modelMap);
const MODELS_RESPONSE = {
  object: "list",
  data: codexModels.map((id) => ({ id, object: "model", created: 1766332800, owned_by: "openai" })),
};

function jsonResponse(res, data, status = 200) {
  res.writeHead(status, { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*" });
  res.end(JSON.stringify(data));
}

function proxyToBackend(req, res) {
  const chunks = [];
  req.on("data", (c) => chunks.push(c));
  req.on("end", () => {
    let body;
    try { body = JSON.parse(Buffer.concat(chunks).toString()); } catch (e) {
      return jsonResponse(res, { error: { message: "invalid JSON" } }, 400);
    }

    const codexModel = body.model || "";
    const mapping = modelMap[codexModel];
    const targetBackend = mapping ? mapping.b : Object.keys(backends)[0];
    const targetModel = mapping ? mapping.m : body.model;
    const targetPort = backends[targetBackend];
    const targetKey = backendKeys[targetBackend] || "";

    if (!targetPort) {
      return jsonResponse(res, { error: { message: `no backend for model: ${codexModel}` } }, 400);
    }

    if (mapping) {
      body.model = targetModel;
      console.log(`[gateway] ${codexModel} → ${targetBackend}:${targetModel} (:${targetPort})`);
    } else {
      console.log(`[gateway] ${codexModel} → default :${targetPort}`);
    }

    const bodyStr = JSON.stringify(body);
    const fwdHeaders = { ...req.headers, host: `${HOST}:${targetPort}` };
    delete fwdHeaders["content-length"];
    if (targetKey) fwdHeaders["authorization"] = `Bearer ${targetKey}`;

    const opts = { hostname: HOST, port: targetPort, path: "/v1/responses", method: req.method, headers: fwdHeaders };
    const backendReq = http.request(opts, (backendRes) => {
      res.writeHead(backendRes.statusCode, { ...backendRes.headers, "Access-Control-Allow-Origin": "*" });
      backendRes.pipe(res);
    });
    backendReq.on("error", (err) => {
      console.error(`[gateway] backend error: ${err.message}`);
      jsonResponse(res, { error: { message: "backend unavailable", type: "proxy_error" } }, 502);
    });
    backendReq.write(bodyStr);
    backendReq.end();
  });
}

const server = http.createServer((req, res) => {
  if (req.method === "OPTIONS") {
    res.writeHead(204, { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Methods": "OPTIONS, POST, GET", "Access-Control-Allow-Headers": "Content-Type, Authorization" });
    return res.end();
  }
  const path = new URL(req.url, `http://${req.headers.host}`).pathname.replace(/\/+$/, "") || "/";

  if (req.method === "GET" && (path === "/v1/models" || path === "/models")) {
    console.log(`[gateway] GET /v1/models`);
    return jsonResponse(res, MODELS_RESPONSE);
  }
  if (req.method === "GET" && (path === "/health" || path === "/v1/health")) {
    return jsonResponse(res, { status: "ok", modelMap, backends });
  }
  proxyToBackend(req, res);
});

server.listen(PORT, HOST, () => {
  console.log(`[gateway] :${PORT} → backends:`, backends);
  console.log(`[gateway] model map:`, modelMap);
  console.log(`[gateway] visible models:`, codexModels);
});
