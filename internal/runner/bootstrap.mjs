import { appendFileSync, writeFileSync, readFileSync } from "node:fs";
import { handler } from "./function.mjs";

const LOG_PATH = "faas_user_logs.ndjson";
const logCaptureRaw = process.env.LOWCODE_FAAS_LOG_CAPTURE;
const logCapture =
  logCaptureRaw === "1" ||
  logCaptureRaw === "true" ||
  logCaptureRaw === "yes";
const runId = process.env.LOWCODE_FAAS_RUN_ID ?? "";
const executionId = process.env.LOWCODE_FAAS_EXECUTION_ID || runId;
const deploymentId = process.env.LOWCODE_FAAS_DEPLOYMENT_ID ?? "";
const collectionPath = process.env.LOWCODE_FAAS_COLLECTION_PATH ?? "";
const deploymentVersionRaw = process.env.LOWCODE_FAAS_DEPLOYMENT_VERSION ?? "";
const fnName = process.env.LOWCODE_FAAS_FUNCTION_NAME ?? "";

function installLogCapture() {
  if (!logCapture) {
    return;
  }
  const orig = {
    log: console.log.bind(console),
    info: console.info.bind(console),
    warn: console.warn.bind(console),
    error: console.error.bind(console),
    debug: console.debug.bind(console),
  };
  const serialize = (args) =>
    args
      .map((a) => {
        try {
          if (typeof a === "string") return a;
          return JSON.stringify(a);
        } catch {
          return String(a);
        }
      })
      .join(" ");
  let deploymentVersion = null;
  if (deploymentVersionRaw !== "") {
    const n = Number(deploymentVersionRaw);
    if (Number.isFinite(n)) deploymentVersion = n;
  }
  const tee = (level, origFn, args) => {
    const rec = {
      ts: new Date().toISOString(),
      level,
      run_id: runId,
      execution_id: executionId,
      deployment_id: deploymentId,
      collection_path: collectionPath,
      function_name: fnName,
      message: serialize(args),
    };
    if (deploymentVersion !== null) {
      rec.deployment_version = deploymentVersion;
    }
    try {
      appendFileSync(LOG_PATH, `${JSON.stringify(rec)}\n`, { flag: "a" });
    } catch {
      // ignore
    }
    origFn(...args);
  };
  console.log = (...a) => tee("log", orig.log, a);
  console.info = (...a) => tee("info", orig.info, a);
  console.warn = (...a) => tee("warn", orig.warn, a);
  console.error = (...a) => tee("error", orig.error, a);
  console.debug = (...a) => tee("debug", orig.debug, a);
}

installLogCapture();

async function main() {
  let input = {};
  try {
    input = JSON.parse(readFileSync("input.json", "utf8"));
  } catch {
    writeFileSync(
      "output.json",
      JSON.stringify({ ok: false, error: "invalid input.json", code: "BAD_INPUT" }),
    );
    process.exit(1);
  }
  if (typeof handler !== "function") {
    writeFileSync(
      "output.json",
      JSON.stringify({ ok: false, error: "export async function handler(input) { ... }", code: "NO_HANDLER" }),
    );
    process.exit(1);
  }
  try {
    const out = await handler(input);
    if (!out || typeof out !== "object" || typeof out.ok !== "boolean") {
      writeFileSync(
        "output.json",
        JSON.stringify({
          ok: false,
          error: "handler must return an object with boolean field ok",
          code: "BAD_RETURN",
        }),
      );
      process.exit(1);
    }
    writeFileSync("output.json", JSON.stringify(out));
  } catch (e) {
    const msg = e && typeof e === "object" && "message" in e ? String(e.message) : String(e);
    writeFileSync(
      "output.json",
      JSON.stringify({ ok: false, error: msg, code: "HANDLER_THROW" }),
    );
    process.exit(1);
  }
}

main();
