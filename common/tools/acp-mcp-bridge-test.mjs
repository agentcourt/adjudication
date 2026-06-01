#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { createConnection } from 'node:net'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createInterface } from 'node:readline'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '..', '..')
const bridgePath = join(here, 'acp-mcp-bridge.mjs')
const token = 'agentcourt-test-token'

async function main() {
  const cfg = parseArgs(process.argv.slice(2))
  const tmp = await mkdtemp(join(tmpdir(), 'agentcourt-acp-mcp-direct-'))
  const fakeFetchOpenClaw = join(tmp, 'fake-openclaw-fetch.mjs')
  const fakeSDKOpenClaw = join(tmp, 'fake-openclaw-sdk.mjs')
  const fakeOpenClawLog = join(tmp, 'fake-openclaw-sessions.txt')
  await writeFile(fakeFetchOpenClaw, fakeFetchOpenClawSource, 'utf8')
  await writeFile(fakeSDKOpenClaw, fakeSDKOpenClawSource, 'utf8')
  let child
  let socket
  try {
    child = spawnService(cfg, tmp, fakeFetchOpenClaw, fakeOpenClawLog)
    const { acpPort } = await waitForListening(child)
    socket = createConnection({ host: '127.0.0.1', port: acpPort })
    const rpc = new JSONLineRPC(socket)
    const calls = []
    rpc.onRequest = async (msg) => {
      if (msg.method !== '_test/probe') {
        throw new Error(`unexpected ACP method ${msg.method}`)
      }
      if (msg.params?.value !== 'first' && msg.params?.value !== 'second') {
        throw new Error(`unexpected probe value ${JSON.stringify(msg.params)}`)
      }
      calls.push(msg.params.value)
      return { text: 'probe ok', echoed: msg.params.value }
    }
    await once(socket, 'connect')
    await rpc.request('initialize', { protocolVersion: 1 })
    const firstSession = await rpc.request('session/new', { cwd: repoRoot, mcpServers: [] })
    const firstPrompt = await rpc.request('session/prompt', {
      sessionId: firstSession.sessionId,
      prompt: [{ type: 'text', text: 'Call the first probe tool.' }],
      _meta: {
        clientTools: [{
          method: '_test/probe',
          toolName: 'aar_probe_first',
          description: 'Probe the ACP client bridge first.',
          parameters: {
            type: 'object',
            properties: { value: { type: 'string' } },
            required: ['value'],
            additionalProperties: false
          }
        }]
      }
    })
    if (firstPrompt.stopReason !== 'end_turn') {
      throw new Error(`unexpected first stopReason ${JSON.stringify(firstPrompt)}`)
    }
    const secondSession = await rpc.request('session/new', { cwd: repoRoot, mcpServers: [] })
    const secondPrompt = await rpc.request('session/prompt', {
      sessionId: secondSession.sessionId,
      prompt: [{ type: 'text', text: 'Call the second probe tool.' }],
      _meta: {
        clientTools: [{
          method: '_test/probe',
          toolName: 'aar_probe_second',
          description: 'Probe the ACP client bridge second.',
          parameters: {
            type: 'object',
            properties: { value: { type: 'string' } },
            required: ['value'],
            additionalProperties: false
          }
        }]
      }
    })
    if (secondPrompt.stopReason !== 'end_turn') {
      throw new Error(`unexpected second stopReason ${JSON.stringify(secondPrompt)}`)
    }
    if (JSON.stringify(calls) !== JSON.stringify(['first', 'second'])) {
      throw new Error(`ACP calls = ${JSON.stringify(calls)}, want ["first","second"]`)
    }
    const sessionIDs = (await readFile(fakeOpenClawLog, 'utf8')).trim().split(/\n+/)
    if (sessionIDs.length !== 2) {
      throw new Error(`OpenClaw invocation count = ${sessionIDs.length}, want 2`)
    }
    if (!sessionIDs[0] || sessionIDs[0] !== sessionIDs[1]) {
      throw new Error(`OpenClaw session ids were not stable: ${JSON.stringify(sessionIDs)}`)
    }
    socket.end()
    child.kill('SIGTERM')
    process.stdout.write(cfg.dockerImage ? 'direct bridge docker MCP test ok\n' : 'direct bridge test ok\n')
  } finally {
    if (socket) socket.destroy()
    if (child && child.exitCode === null) child.kill('SIGTERM')
    await rm(tmp, { recursive: true, force: true })
  }
}

function parseArgs(argv) {
  const cfg = { dockerImage: '' }
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]
    if (arg === '--docker-image') {
      const value = argv[i + 1]
      if (!value) throw new Error('--docker-image requires a value')
      cfg.dockerImage = value
      i += 1
      continue
    }
    if (arg === '--help' || arg === '-h') {
      process.stdout.write('Usage: common/tools/acp-mcp-bridge-test.mjs [--docker-image IMAGE]\n')
      process.exit(0)
    }
    throw new Error(`unknown argument: ${arg}`)
  }
  return cfg
}

function spawnService(cfg, tmp, fakeFetchOpenClaw, fakeOpenClawLog) {
  const env = {
    ...process.env,
    AGENTCOURT_OPENCLAW_COMMAND: process.execPath,
    AGENTCOURT_OPENCLAW_BASE_ARGS_JSON: JSON.stringify([fakeFetchOpenClaw]),
    AGENTCOURT_OPENCLAW_TIMEOUT_SECONDS: '20',
    AGENTCOURT_OPENCLAW_AGENT_ID: 'test-lawyer',
    AGENTCOURT_OPENCLAW_MCP_SERVER_NAME: 'agentcourt',
    FAKE_OPENCLAW_LOG: fakeOpenClawLog
  }
  if (cfg.dockerImage) {
    env.AGENTCOURT_OPENCLAW_COMMAND = 'docker'
    env.AGENTCOURT_OPENCLAW_BASE_ARGS_JSON = JSON.stringify([
      'run',
      '--rm',
      '--network',
      'host',
      '-v',
      `${tmp}:/agentcourt-test:rw`,
      '-e',
      'AGENTCOURT_OPENCLAW_MCP_URL',
      '-e',
      'AGENTCOURT_OPENCLAW_MCP_TOKEN',
      '-e',
      'FAKE_OPENCLAW_LOG=/agentcourt-test/fake-openclaw-sessions.txt',
      cfg.dockerImage,
      'node',
      '/agentcourt-test/fake-openclaw-sdk.mjs'
    ])
  }
  return spawn(process.execPath, [
    bridgePath,
    '--acp-host',
    '127.0.0.1',
    '--acp-port',
    '0',
    '--mcp-host',
    '127.0.0.1',
    '--mcp-port',
    '0',
    '--token',
    token
  ], {
    cwd: repoRoot,
    env,
    stdio: ['ignore', 'ignore', 'pipe']
  })
}

function waitForListening(child) {
  const stderr = createInterface({ input: child.stderr, crlfDelay: Infinity })
  const state = { acpPort: 0, mcpURL: '' }
  let stderrText = ''
  return new Promise((resolve, reject) => {
    const finish = () => {
      if (state.acpPort && state.mcpURL) resolve({ ...state })
    }
    child.once('error', reject)
    child.once('exit', (code, signal) => {
      reject(new Error(`bridge exited before listen code=${code} signal=${signal}: ${stderrText.trim()}`))
    })
    stderr.on('line', (line) => {
      stderrText += `${line}\n`
      const acp = line.match(/acp listening on tcp:\/\/127\.0\.0\.1:(\d+)/)
      if (acp) state.acpPort = Number(acp[1])
      const mcp = line.match(/mcp listening on (http:\/\/127\.0\.0\.1:\d+\/mcp)/)
      if (mcp) state.mcpURL = mcp[1]
      finish()
    })
  })
}

class JSONLineRPC {
  constructor(socket) {
    this.socket = socket
    this.nextID = 0
    this.pending = new Map()
    this.onRequest = null
    const lines = createInterface({ input: socket, crlfDelay: Infinity })
    lines.on('line', (line) => {
      void this.handleLine(line).catch((err) => {
        for (const pending of this.pending.values()) pending.reject(err)
        this.pending.clear()
      })
    })
    socket.on('error', (err) => {
      for (const pending of this.pending.values()) pending.reject(err)
      this.pending.clear()
    })
  }

  request(method, params) {
    const id = this.nextID += 1
    const promise = new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
    })
    this.socket.write(`${JSON.stringify({ jsonrpc: '2.0', id, method, params })}\n`)
    return promise
  }

  async handleLine(line) {
    const msg = JSON.parse(line)
    if (typeof msg.method === 'string' && Object.hasOwn(msg, 'id')) {
      try {
        const result = await this.onRequest(msg)
        this.socket.write(`${JSON.stringify({ jsonrpc: '2.0', id: msg.id, result })}\n`)
      } catch (err) {
        this.socket.write(`${JSON.stringify({ jsonrpc: '2.0', id: msg.id, error: { code: -32000, message: errorText(err) } })}\n`)
      }
      return
    }
    const pending = this.pending.get(Number(msg.id))
    if (!pending) return
    this.pending.delete(Number(msg.id))
    if (msg.error) {
      pending.reject(new Error(String(msg.error.message || 'request failed')))
      return
    }
    pending.resolve(msg.result)
  }
}

function once(emitter, event) {
  return new Promise((resolve, reject) => {
    emitter.once(event, resolve)
    emitter.once('error', reject)
  })
}

function errorText(err) {
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err) return err
  return String(err)
}

const fakeFetchOpenClawSource = `
import { appendFile } from "node:fs/promises";

const args = process.argv.slice(2);
const sessionIDIndex = args.indexOf("--session-id");
if (sessionIDIndex < 0 || !args[sessionIDIndex + 1]) {
  throw new Error("missing --session-id");
}
await appendFile(process.env.FAKE_OPENCLAW_LOG, args[sessionIDIndex + 1] + "\\n");
const { expectedTool, rejectedTool, value } = expectedToolInfo(args);
const mcpURL = process.env.AGENTCOURT_OPENCLAW_MCP_URL;
const token = process.env.AGENTCOURT_OPENCLAW_MCP_TOKEN;
if (!mcpURL || !token) {
  throw new Error("missing MCP URL or token");
}

await rpc(mcpURL, token, 1, "initialize", {
  protocolVersion: "2025-06-18",
  capabilities: {},
  clientInfo: { name: "fake-openclaw", version: "0.1.0" }
});
await rpc(mcpURL, token, undefined, "notifications/initialized", {});
const listed = await rpc(mcpURL, token, 2, "tools/list", {});
if (!Array.isArray(listed.tools) || !listed.tools.some((tool) => tool.name === expectedTool)) {
  throw new Error(expectedTool + " was not exposed");
}
if (listed.tools.some((tool) => tool.name === rejectedTool)) {
  throw new Error(rejectedTool + " leaked into this prompt");
}
const called = await rpc(mcpURL, token, 3, "tools/call", {
  name: expectedTool,
  arguments: { value }
});
if (called.isError) {
  throw new Error("tool call returned error: " + JSON.stringify(called));
}
process.stdout.write(JSON.stringify({ ok: true }) + "\\n");

function expectedToolInfo(args) {
  const messageIndex = args.indexOf("--message");
  const message = messageIndex >= 0 ? args[messageIndex + 1] || "" : "";
  const second = message.includes("second probe");
  return {
    expectedTool: second ? "aar_probe_second" : "aar_probe_first",
    rejectedTool: second ? "aar_probe_first" : "aar_probe_second",
    value: second ? "second" : "first"
  };
}

async function rpc(url, token, id, method, params) {
  const body = id === undefined
    ? { jsonrpc: "2.0", method, params }
    : { jsonrpc: "2.0", id, method, params };
  const response = await fetch(url, {
    method: "POST",
    headers: {
      authorization: "Bearer " + token,
      "content-type": "application/json",
      accept: "application/json, text/event-stream"
    },
    body: JSON.stringify(body)
  });
  if (id === undefined) {
    if (response.status !== 202) {
      throw new Error("notification response status " + response.status + ": " + await response.text());
    }
    return {};
  }
  const payload = await response.json();
  if (!response.ok || payload.error) {
    throw new Error("MCP request failed: " + JSON.stringify(payload));
  }
  return payload.result;
}
`

const fakeSDKOpenClawSource = `
import { appendFile } from "node:fs/promises";
import { Client } from "file:///app/node_modules/@modelcontextprotocol/sdk/dist/esm/client/index.js";
import { StreamableHTTPClientTransport } from "file:///app/node_modules/@modelcontextprotocol/sdk/dist/esm/client/streamableHttp.js";

const args = process.argv.slice(2);
const sessionIDIndex = args.indexOf("--session-id");
if (sessionIDIndex < 0 || !args[sessionIDIndex + 1]) {
  throw new Error("missing --session-id");
}
await appendFile(process.env.FAKE_OPENCLAW_LOG, args[sessionIDIndex + 1] + "\\n");
const { expectedTool, rejectedTool, value } = expectedToolInfo(args);
const mcpURL = process.env.AGENTCOURT_OPENCLAW_MCP_URL;
const token = process.env.AGENTCOURT_OPENCLAW_MCP_TOKEN;
if (!mcpURL || !token) {
  throw new Error("missing MCP URL or token");
}

const client = new Client(
  { name: "fake-openclaw-sdk", version: "0.1.0" },
  { capabilities: {} }
);
const transport = new StreamableHTTPClientTransport(new URL(mcpURL), {
  requestInit: { headers: { authorization: "Bearer " + token } }
});
await client.connect(transport);
const listed = await client.listTools();
if (!Array.isArray(listed.tools) || !listed.tools.some((tool) => tool.name === expectedTool)) {
  throw new Error(expectedTool + " was not exposed");
}
if (listed.tools.some((tool) => tool.name === rejectedTool)) {
  throw new Error(rejectedTool + " leaked into this prompt");
}
const called = await client.callTool({
  name: expectedTool,
  arguments: { value }
});
if (called.isError) {
  throw new Error("tool call returned error: " + JSON.stringify(called));
}
await client.close();
process.stdout.write(JSON.stringify({ ok: true }) + "\\n");

function expectedToolInfo(args) {
  const messageIndex = args.indexOf("--message");
  const message = messageIndex >= 0 ? args[messageIndex + 1] || "" : "";
  const second = message.includes("second probe");
  return {
    expectedTool: second ? "aar_probe_second" : "aar_probe_first",
    rejectedTool: second ? "aar_probe_first" : "aar_probe_second",
    value: second ? "second" : "first"
  };
}
`

main().catch((err) => {
  process.stderr.write(`${errorText(err)}\n`)
  process.exit(1)
})
