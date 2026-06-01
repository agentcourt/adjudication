#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { createServer as createHTTPServer } from 'node:http'
import { createServer as createTCPServer } from 'node:net'
import { createInterface } from 'node:readline'

const VERSION = '0.2.0'
const DEFAULT_SCHEMA = { type: 'object', properties: {}, additionalProperties: true }

const usage = `Usage: common/tools/acp-mcp-bridge.mjs [options]

Run one OpenClaw-backed ACP lawyer service.  The service listens for ACP over
TCP and exposes the current ACP client tools to OpenClaw through MCP over HTTP.

Options:
  --acp-host HOST              ACP TCP bind host, default 127.0.0.1
  --acp-port PORT              ACP TCP bind port, default 19701
  --mcp-host HOST              MCP HTTP bind host, default 127.0.0.1
  --mcp-port PORT              MCP HTTP bind port, default 19702
  --mcp-path PATH              MCP HTTP path, default /mcp
  --token TOKEN                MCP bearer token, default random
  --openclaw COMMAND           OpenClaw command, default openclaw
  --agent-id ID                OpenClaw agent id
  --session-id ID              OpenClaw session id, default generated once
  --timeout-seconds SECONDS    OpenClaw command timeout, default 900
  --tool-timeout-seconds SEC   ACP client tool timeout, default 120
  --mcp-server-name NAME       OpenClaw MCP server name, default agentcourt
`

function parseArgs(argv) {
  const cfg = {
    acpHost: process.env.AGENTCOURT_ACP_HOST || '127.0.0.1',
    acpPort: process.env.AGENTCOURT_ACP_PORT || '19701',
    mcpHost: process.env.AGENTCOURT_MCP_HOST || '127.0.0.1',
    mcpPort: process.env.AGENTCOURT_MCP_PORT || '19702',
    mcpPath: process.env.AGENTCOURT_MCP_PATH || '/mcp',
    token: process.env.AGENTCOURT_MCP_TOKEN || '',
    openclawCommand: process.env.AGENTCOURT_OPENCLAW_COMMAND || process.env.AGENTCOURT_OPENCLAW_CLI || 'openclaw',
    agentID: process.env.AGENTCOURT_OPENCLAW_AGENT_ID || '',
    sessionID: process.env.AGENTCOURT_OPENCLAW_SESSION_ID || '',
    thinking: process.env.AGENTCOURT_OPENCLAW_THINKING || '',
    local: envBool(process.env.AGENTCOURT_OPENCLAW_LOCAL),
    cwd: process.env.AGENTCOURT_OPENCLAW_CWD || '',
    timeoutSeconds: process.env.AGENTCOURT_OPENCLAW_TIMEOUT_SECONDS || '900',
    toolTimeoutSeconds: process.env.AGENTCOURT_ACP_TOOL_TIMEOUT_SECONDS || '120',
    extraPrompt: process.env.AGENTCOURT_OPENCLAW_EXTRA_PROMPT || '',
    mcpServerName: process.env.AGENTCOURT_OPENCLAW_MCP_SERVER_NAME || 'agentcourt'
  }
  const optionMap = {
    '--acp-host': 'acpHost',
    '--acp-port': 'acpPort',
    '--mcp-host': 'mcpHost',
    '--mcp-port': 'mcpPort',
    '--mcp-path': 'mcpPath',
    '--token': 'token',
    '--openclaw': 'openclawCommand',
    '--agent-id': 'agentID',
    '--session-id': 'sessionID',
    '--timeout-seconds': 'timeoutSeconds',
    '--tool-timeout-seconds': 'toolTimeoutSeconds',
    '--mcp-server-name': 'mcpServerName'
  }
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]
    if (arg === '--help' || arg === '-h') {
      process.stdout.write(usage)
      process.exit(0)
    }
    if (!Object.hasOwn(optionMap, arg)) {
      throw new Error(`unknown argument: ${arg}`)
    }
    const value = argv[i + 1]
    if (!value) throw new Error(`${arg} requires a value`)
    cfg[optionMap[arg]] = value
    i += 1
  }
  cfg.acpPort = parsePort(cfg.acpPort, 'ACP port')
  cfg.mcpPort = parsePort(cfg.mcpPort, 'MCP port')
  cfg.timeoutSeconds = parsePositiveInt(cfg.timeoutSeconds, 'OpenClaw timeout seconds')
  cfg.toolTimeoutSeconds = parsePositiveInt(cfg.toolTimeoutSeconds, 'ACP tool timeout seconds')
  cfg.mcpPath = normalizePath(cfg.mcpPath)
  cfg.mcpServerName = safeServerName(cfg.mcpServerName || 'agentcourt')
  if (!cfg.token) cfg.token = randomBytes(24).toString('hex')
  return cfg
}

function parsePort(raw, label) {
  const port = Number(raw)
  if (!Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error(`invalid ${label}: ${raw}`)
  }
  return port
}

function parsePositiveInt(raw, label) {
  const value = Number(raw)
  if (!Number.isInteger(value) || value < 1) {
    throw new Error(`invalid ${label}: ${raw}`)
  }
  return value
}

function normalizePath(path) {
  const text = String(path || '').trim()
  if (!text || !text.startsWith('/')) throw new Error(`invalid MCP path: ${path}`)
  return text.replace(/\/+$/, '') || '/mcp'
}

function envBool(raw) {
  return raw === '1' || raw === 'true' || raw === 'yes'
}

function safeServerName(name) {
  const cleaned = String(name).trim().replace(/[^a-zA-Z0-9_-]+/g, '_').replace(/^_+|_+$/g, '')
  return cleaned || 'agentcourt'
}

class BridgeService {
  constructor(cfg) {
    this.cfg = cfg
    this.connections = new Set()
    this.activePrompt = null
    this.openclawSessionID = cfg.sessionID || `agentcourt-openclaw-${Date.now()}-${randomBytes(6).toString('hex')}`
    this.tcpServer = createTCPServer((socket) => this.acceptACP(socket))
    this.httpServer = createHTTPServer((req, res) => {
      void this.handleHTTP(req, res).catch((err) => {
        writeJSON(res, 500, { error: errorText(err) })
      })
    })
  }

  async run() {
    await Promise.all([this.listenACP(), this.listenMCP()])
    process.on('SIGINT', () => this.shutdown('SIGINT'))
    process.on('SIGTERM', () => this.shutdown('SIGTERM'))
    await new Promise(() => {})
  }

  listenACP() {
    return new Promise((resolve, reject) => {
      this.tcpServer.once('error', reject)
      this.tcpServer.listen(this.cfg.acpPort, this.cfg.acpHost, () => {
        const address = this.tcpServer.address()
        if (!address || typeof address === 'string') {
          reject(new Error('could not determine ACP listener address'))
          return
        }
        this.cfg.acpPort = address.port
        this.tcpServer.removeListener('error', reject)
        log(`acp listening on tcp://${this.cfg.acpHost}:${address.port}`)
        resolve()
      })
    })
  }

  listenMCP() {
    return new Promise((resolve, reject) => {
      this.httpServer.once('error', reject)
      this.httpServer.listen(this.cfg.mcpPort, this.cfg.mcpHost, () => {
        const address = this.httpServer.address()
        if (!address || typeof address === 'string') {
          reject(new Error('could not determine MCP listener address'))
          return
        }
        this.cfg.mcpPort = address.port
        this.httpServer.removeListener('error', reject)
        log(`mcp listening on ${this.mcpURL()}`)
        resolve()
      })
    })
  }

  mcpURL() {
    return `http://${this.cfg.mcpHost}:${this.cfg.mcpPort}${this.cfg.mcpPath}`
  }

  acceptACP(socket) {
    const conn = new ACPConnection(this, socket)
    this.connections.add(conn)
    socket.on('close', () => {
      this.connections.delete(conn)
      conn.close()
      if (this.activePrompt?.conn === conn) this.activePrompt = null
    })
    socket.on('error', (err) => {
      log(`ACP socket error: ${errorText(err)}`)
    })
    conn.run()
  }

  beginPrompt(conn, tools) {
    if (this.activePrompt && this.activePrompt.conn !== conn) {
      throw new Error('another ACP prompt is already running')
    }
    this.activePrompt = { conn, tools, byName: toolMap(tools) }
  }

  endPrompt(conn) {
    if (this.activePrompt?.conn === conn) this.activePrompt = null
  }

  async runOpenClaw(prompt, tools) {
    const message = buildOpenClawPrompt(prompt, tools, this.cfg)
    const args = openclawArgs(this.cfg, message, this.openclawSessionID)
    const env = {
      ...process.env,
      AGENTCOURT_OPENCLAW_MCP_URL: this.mcpURL(),
      AGENTCOURT_OPENCLAW_MCP_TOKEN: this.cfg.token
    }
    const child = spawn(this.cfg.openclawCommand, args, {
      cwd: this.cfg.cwd || undefined,
      env,
      stdio: ['ignore', 'pipe', 'pipe']
    })
    const stdout = []
    const stderr = []
    child.stdout.on('data', (chunk) => stdout.push(Buffer.from(chunk)))
    child.stderr.on('data', (chunk) => stderr.push(Buffer.from(chunk)))
    let timedOut = false
    const timer = setTimeout(() => {
      timedOut = true
      child.kill('SIGTERM')
    }, this.cfg.timeoutSeconds * 1000)
    const result = await new Promise((resolve, reject) => {
      child.once('error', reject)
      child.once('exit', (code, signal) => resolve({ code, signal }))
    }).finally(() => clearTimeout(timer))
    const stderrText = Buffer.concat(stderr).toString('utf8').trim()
    if (timedOut) throw new Error(`OpenClaw command timed out after ${this.cfg.timeoutSeconds}s`)
    if (result.code !== 0) {
      const detail = stderrText ? `: ${stderrText}` : ''
      throw new Error(`OpenClaw command exited with code ${result.code}${detail}`)
    }
    if (result.signal) {
      const detail = stderrText ? `: ${stderrText}` : ''
      throw new Error(`OpenClaw command exited on signal ${result.signal}${detail}`)
    }
    if (stderrText) log(stderrText)
    return Buffer.concat(stdout).toString('utf8')
  }

  async handleHTTP(req, res) {
    const url = new URL(req.url || '/', `http://${req.headers.host || '127.0.0.1'}`)
    if (req.method === 'GET' && url.pathname === '/health') {
      writeJSON(res, 200, { ok: true })
      return
    }
    if (url.pathname !== this.cfg.mcpPath) {
      writeJSON(res, 404, { error: 'not found' })
      return
    }
    if (!this.authorized(req)) {
      writeJSON(res, 403, { error: 'invalid MCP token' })
      return
    }
    if (req.method === 'GET' || req.method === 'DELETE') {
      writeJSON(res, 405, { error: 'method not allowed' })
      return
    }
    if (req.method !== 'POST') {
      writeJSON(res, 405, { error: 'method not allowed' })
      return
    }
    const body = await readJSONBody(req)
    const messages = Array.isArray(body) ? body : [body]
    const responses = []
    for (const msg of messages) {
      const response = await this.handleMCPMessage(msg)
      if (response) responses.push(response)
    }
    if (responses.length === 0) {
      res.statusCode = 202
      res.end()
      return
    }
    writeJSON(res, 200, Array.isArray(body) ? responses : responses[0])
  }

  authorized(req) {
    const header = String(req.headers.authorization || '')
    return header === `Bearer ${this.cfg.token}`
  }

  async handleMCPMessage(msg) {
    if (!msg || typeof msg !== 'object') {
      return mcpError(null, -32600, 'invalid JSON-RPC message')
    }
    const id = Object.hasOwn(msg, 'id') ? msg.id : undefined
    const method = typeof msg.method === 'string' ? msg.method : ''
    if (!method) {
      return id === undefined ? null : mcpError(id, -32600, 'missing JSON-RPC method')
    }
    try {
      switch (method) {
      case 'initialize':
        return mcpResult(id, {
          protocolVersion: typeof msg.params?.protocolVersion === 'string' ? msg.params.protocolVersion : '2025-06-18',
          capabilities: { tools: {} },
          serverInfo: { name: 'agentcourt-openclaw-acp-mcp-bridge', version: VERSION }
        })
      case 'notifications/initialized':
        return null
      case 'ping':
        return mcpResult(id, {})
      case 'tools/list':
        return mcpResult(id, { tools: this.currentMCPTools() })
      case 'tools/call':
        return mcpResult(id, await this.callMCPTool(msg.params || {}))
      default:
        return id === undefined ? null : mcpError(id, -32601, `method not handled: ${method}`)
      }
    } catch (err) {
      return id === undefined ? null : mcpError(id, -32000, errorText(err))
    }
  }

  currentMCPTools() {
    const prompt = this.activePrompt
    if (!prompt) return []
    return prompt.tools.map((tool) => ({
      name: tool.name,
      description: tool.description,
      inputSchema: tool.inputSchema
    }))
  }

  async callMCPTool(params) {
    const prompt = this.activePrompt
    if (!prompt) {
      return { content: [{ type: 'text', text: 'no active ACP prompt' }], isError: true }
    }
    const name = typeof params.name === 'string' ? params.name : ''
    const tool = prompt.byName.get(name)
    if (!tool) {
      return { content: [{ type: 'text', text: `unknown client tool: ${name || '(empty)'}` }], isError: true }
    }
    const args = params.arguments && typeof params.arguments === 'object' && !Array.isArray(params.arguments) ? params.arguments : {}
    try {
      const result = await prompt.conn.callACPMethod(tool.method, args)
      return { content: contentFromResult(result), isError: false }
    } catch (err) {
      return { content: [{ type: 'text', text: errorText(err) }], isError: true }
    }
  }

  shutdown(signal) {
    log(`received ${signal}; shutting down`)
    for (const conn of this.connections) conn.close()
    this.tcpServer.close()
    this.httpServer.closeAllConnections?.()
    this.httpServer.closeIdleConnections?.()
    this.httpServer.close()
    setTimeout(() => process.exit(0), 250).unref()
  }
}

class ACPConnection {
  constructor(service, socket) {
    this.service = service
    this.socket = socket
    this.nextID = 0
    this.pending = new Map()
    this.sessions = new Map()
    this.closed = false
  }

  run() {
    const lines = createInterface({ input: this.socket, crlfDelay: Infinity })
    lines.on('line', (line) => {
      void this.handleLine(line).catch((err) => {
        log(`ACP request error: ${errorText(err)}`)
      })
    })
  }

  async handleLine(line) {
    const text = line.trim()
    if (!text) return
    let msg
    try {
      msg = JSON.parse(text)
    } catch (err) {
      throw new Error(`invalid ACP JSON: ${errorText(err)}`)
    }
    if (typeof msg.method === 'string') {
      if (Object.hasOwn(msg, 'id')) await this.handleRequest(msg)
      return
    }
    this.handleResponse(msg)
  }

  async handleRequest(msg) {
    try {
      switch (msg.method) {
      case 'initialize':
        await this.writeResult(msg.id, {
          protocolVersion: Number(msg.params?.protocolVersion) || 1,
          agentInfo: {
            name: 'agentcourt-openclaw-acp-mcp-bridge',
            title: 'Agent Court OpenClaw ACP MCP Bridge',
            version: VERSION
          }
        })
        return
      case 'session/new': {
        const sessionID = `agentcourt-openclaw-${Date.now()}-${this.sessions.size + 1}`
        this.sessions.set(sessionID, { cwd: typeof msg.params?.cwd === 'string' ? msg.params.cwd : '' })
        await this.writeResult(msg.id, { sessionId: sessionID })
        return
      }
      case 'session/prompt':
        await this.handlePrompt(msg.id, msg.params || {})
        return
      default:
        await this.writeError(msg.id, -32601, `method not handled: ${msg.method}`)
      }
    } catch (err) {
      await this.writeError(msg.id, -32000, errorText(err))
    }
  }

  handleResponse(msg) {
    const key = String(msg.id)
    const pending = this.pending.get(key)
    if (!pending) return
    this.pending.delete(key)
    clearTimeout(pending.timeout)
    if (msg.error) {
      pending.reject(new Error(String(msg.error.message || 'ACP client method failed')))
      return
    }
    pending.resolve(msg.result ?? {})
  }

  async handlePrompt(id, params) {
    const sessionID = typeof params.sessionId === 'string' ? params.sessionId : ''
    if (sessionID && !this.sessions.has(sessionID)) {
      throw new Error(`unknown ACP session: ${sessionID}`)
    }
    const tools = loadPromptClientTools(params?._meta).map(normalizeToolSpec)
    this.service.beginPrompt(this, tools)
    try {
      const prompt = promptText(params?.prompt)
      await this.service.runOpenClaw(prompt, tools)
      await this.writeResult(id, { stopReason: 'end_turn' })
    } finally {
      this.service.endPrompt(this)
    }
  }

  callACPMethod(method, params) {
    const id = this.nextID += 1
    const key = String(id)
    const timeout = setTimeout(() => {
      const pending = this.pending.get(key)
      if (!pending) return
      this.pending.delete(key)
      pending.reject(new Error(`ACP client method ${method} timed out after ${this.service.cfg.toolTimeoutSeconds}s`))
    }, this.service.cfg.toolTimeoutSeconds * 1000)
    const promise = new Promise((resolve, reject) => {
      this.pending.set(key, { resolve, reject, timeout })
    })
    this.write({
      jsonrpc: '2.0',
      id,
      method,
      params: params && typeof params === 'object' && !Array.isArray(params) ? params : {}
    }).catch((err) => {
      const pending = this.pending.get(key)
      if (!pending) return
      this.pending.delete(key)
      clearTimeout(timeout)
      pending.reject(err)
    })
    return promise
  }

  writeResult(id, result) {
    return this.write({ jsonrpc: '2.0', id, result })
  }

  writeError(id, code, message) {
    return this.write({ jsonrpc: '2.0', id, error: { code, message } })
  }

  write(msg) {
    if (this.closed) return Promise.reject(new Error('ACP connection is closed'))
    return new Promise((resolve, reject) => {
      this.socket.write(`${JSON.stringify(msg)}\n`, (err) => {
        if (err) reject(err)
        else resolve()
      })
    })
  }

  close() {
    if (this.closed) return
    this.closed = true
    for (const [id, pending] of this.pending) {
      this.pending.delete(id)
      clearTimeout(pending.timeout)
      pending.reject(new Error('ACP connection closed'))
    }
    this.socket.destroy()
  }
}

function loadPromptClientTools(meta) {
  if (!meta || typeof meta !== 'object') return []
  const raw = meta.clientTools
  if (raw == null) return []
  if (!Array.isArray(raw)) throw new Error('_meta.clientTools must be an array')
  return raw
}

function normalizeToolSpec(item) {
  if (!item || typeof item !== 'object') throw new Error('client tool specs must be objects')
  const method = typeof item.method === 'string' ? item.method.trim() : ''
  if (!method.startsWith('_')) throw new Error(`client tool method must start with _: ${method || '(empty)'}`)
  const name = toolNameFor(item, method)
  return {
    method,
    name,
    description: typeof item.description === 'string' && item.description.trim()
      ? item.description.trim()
      : `Call ACP client method ${method}`,
    inputSchema: item.parameters && typeof item.parameters === 'object' && !Array.isArray(item.parameters)
      ? item.parameters
      : DEFAULT_SCHEMA
  }
}

function toolNameFor(item, method) {
  const explicit = typeof item.toolName === 'string' ? item.toolName.trim() : ''
  const name = explicit || method.replace(/^_+/, '').replace(/[^a-zA-Z0-9]+/g, '_').replace(/^_+|_+$/g, '').toLowerCase()
  if (!name || !/^[a-zA-Z0-9_-]+$/.test(name)) {
    throw new Error(`invalid client tool name: ${name || '(empty)'}`)
  }
  return name
}

function toolMap(tools) {
  const byName = new Map()
  for (const tool of tools) {
    if (byName.has(tool.name)) throw new Error(`duplicate client tool name: ${tool.name}`)
    byName.set(tool.name, tool)
  }
  return byName
}

function promptText(blocks) {
  if (!Array.isArray(blocks)) return ''
  return blocks
    .filter((block) => block && block.type === 'text' && typeof block.text === 'string')
    .map((block) => block.text)
    .join('\n\n')
}

function buildOpenClawPrompt(prompt, tools, cfg) {
  const toolLines = tools.map((tool) => {
    const name = `${cfg.mcpServerName}__${tool.name}`
    return `- ${name}: ${tool.description}`
  })
  const sections = []
  if (toolLines.length > 0) {
    sections.push([
      'Client tools are available through OpenClaw MCP under these names:',
      ...toolLines,
      'Use the relevant client tool to complete the requested task before ending the turn.'
    ].join('\n'))
  }
  if (cfg.extraPrompt.trim()) sections.push(cfg.extraPrompt.trim())
  sections.push(prompt)
  return sections.filter((part) => part.trim()).join('\n\n')
}

function openclawArgs(cfg, message, sessionID) {
  const baseArgs = parseJSONEnvArray('AGENTCOURT_OPENCLAW_BASE_ARGS_JSON')
  const extraArgs = parseJSONEnvArray('AGENTCOURT_OPENCLAW_EXTRA_ARGS_JSON')
  const args = [
    ...baseArgs,
    'agent',
    '--message',
    message,
    '--json',
    '--timeout',
    String(cfg.timeoutSeconds)
  ]
  if (cfg.agentID) args.push('--agent', cfg.agentID)
  if (sessionID) args.push('--session-id', sessionID)
  if (cfg.thinking) args.push('--thinking', cfg.thinking)
  if (cfg.local) args.push('--local')
  args.push(...extraArgs)
  return args
}

function parseJSONEnvArray(name) {
  const raw = process.env[name]
  if (!raw || !raw.trim()) return []
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch (err) {
    throw new Error(`invalid ${name}: ${errorText(err)}`)
  }
  if (!Array.isArray(parsed) || parsed.some((item) => typeof item !== 'string')) {
    throw new Error(`${name} must be a JSON array of strings`)
  }
  return parsed
}

function contentFromResult(result) {
  if (result && typeof result === 'object' && Array.isArray(result.content) && result.content.length > 0) {
    return result.content
  }
  if (result && typeof result === 'object' && typeof result.text === 'string' && result.text) {
    return [{ type: 'text', text: result.text }]
  }
  return [{ type: 'text', text: JSON.stringify(result ?? {}, null, 2) }]
}

async function readJSONBody(req) {
  const chunks = []
  for await (const chunk of req) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  }
  const text = Buffer.concat(chunks).toString('utf8')
  if (!text.trim()) return {}
  return JSON.parse(text)
}

function writeJSON(res, statusCode, payload) {
  res.statusCode = statusCode
  res.setHeader('content-type', 'application/json')
  res.end(JSON.stringify(payload))
}

function mcpResult(id, result) {
  if (id === undefined) return null
  return { jsonrpc: '2.0', id, result }
}

function mcpError(id, code, message) {
  return { jsonrpc: '2.0', id, error: { code, message } }
}

function log(message) {
  if (message) process.stderr.write(`${message}\n`)
}

function errorText(err) {
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err) return err
  if (err && typeof err === 'object') {
    const message = err.message
    if (typeof message === 'string' && message) return message
    try {
      return JSON.stringify(err)
    } catch {
      return String(err)
    }
  }
  return String(err)
}

async function main() {
  const cfg = parseArgs(process.argv.slice(2))
  await new BridgeService(cfg).run()
}

main().catch((err) => {
  process.stderr.write(`${errorText(err)}\n\n${usage}`)
  process.exit(2)
})
