#!/usr/bin/env node
'use strict';

const net = require('net');
const path = require('path');
const { spawn } = require('child_process');

const usage = `Usage: tools/openclaw-acp-tcp-bridge.js [--host HOST] [--port PORT] [--adapter PATH]

Expose the stdio aar-openclaw-attorney adapter as a TCP ACP endpoint.

Defaults:
  --host     127.0.0.1
  --port     19701
  --adapter  .bin/aar-openclaw-attorney

Environment:
  AAR_OPENCLAW_ACP_HOST                   default host
  AAR_OPENCLAW_ACP_PORT                   default port
  AAR_OPENCLAW_ADAPTER                    default adapter path
  AAR_OPENCLAW_AGENT_ID                   adapter agent id, default aar-lawyer
  AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS   adapter timeout, default 900

The bridge intentionally removes AAR_OPENCLAW_AGENT_MODEL from the adapter
environment. Endpoint attorneys own model selection on the OpenClaw side.
`;

function parseArgs(argv) {
  const args = {
    host: process.env.AAR_OPENCLAW_ACP_HOST || '127.0.0.1',
    port: process.env.AAR_OPENCLAW_ACP_PORT || '19701',
    adapter: process.env.AAR_OPENCLAW_ADAPTER || path.join(process.cwd(), '.bin', 'aar-openclaw-attorney'),
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--help' || arg === '-h') {
      process.stdout.write(usage);
      process.exit(0);
    }
    if (arg === '--host' || arg === '--port' || arg === '--adapter') {
      const value = argv[i + 1];
      if (!value) {
        throw new Error(`${arg} requires a value`);
      }
      args[arg.slice(2)] = value;
      i += 1;
      continue;
    }
    throw new Error(`unknown argument: ${arg}`);
  }
  const port = Number(args.port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`invalid port: ${args.port}`);
  }
  args.port = port;
  return args;
}

let cfg;
try {
  cfg = parseArgs(process.argv.slice(2));
} catch (err) {
  process.stderr.write(`${err.message}\n\n${usage}`);
  process.exit(2);
}

let seq = 0;
const children = new Set();

const server = net.createServer((socket) => {
  const id = ++seq;
  const env = {
    ...process.env,
    AAR_OPENCLAW_AGENT: '1',
    AAR_OPENCLAW_AGENT_ID: process.env.AAR_OPENCLAW_AGENT_ID || 'aar-lawyer',
    AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS: process.env.AAR_OPENCLAW_ATTORNEY_TIMEOUT_SECONDS || '900',
  };
  delete env.AAR_OPENCLAW_AGENT_MODEL;

  const child = spawn(cfg.adapter, [], { env, stdio: ['pipe', 'pipe', 'pipe'] });
  children.add(child);
  console.error(`[${new Date().toISOString()}] connection ${id}: spawned ${cfg.adapter} pid=${child.pid}`);

  const closeChild = () => {
    if (!child.killed && child.exitCode === null) {
      child.kill('SIGTERM');
    }
  };

  socket.pipe(child.stdin);
  child.stdout.pipe(socket);
  child.stderr.on('data', (buf) => process.stderr.write(`[${id} stderr] ${buf}`));
  socket.on('error', (err) => console.error(`[${new Date().toISOString()}] connection ${id}: socket error: ${err.message}`));
  socket.on('close', closeChild);
  child.on('error', (err) => {
    console.error(`[${new Date().toISOString()}] connection ${id}: spawn error: ${err.message}`);
    socket.destroy(err);
  });
  child.on('exit', (code, signal) => {
    children.delete(child);
    console.error(`[${new Date().toISOString()}] connection ${id}: child exited code=${code} signal=${signal}`);
    socket.end();
  });
});

server.on('error', (err) => {
  console.error(`[${new Date().toISOString()}] server error: ${err.message}`);
  process.exit(1);
});

function shutdown(signal) {
  console.error(`[${new Date().toISOString()}] received ${signal}; shutting down`);
  server.close(() => process.exit(0));
  for (const child of children) {
    if (!child.killed && child.exitCode === null) {
      child.kill('SIGTERM');
    }
  }
  setTimeout(() => process.exit(0), 3000).unref();
}

process.on('SIGINT', () => shutdown('SIGINT'));
process.on('SIGTERM', () => shutdown('SIGTERM'));

server.listen(cfg.port, cfg.host, () => {
  console.error(`[${new Date().toISOString()}] listening on tcp://${cfg.host}:${cfg.port}`);
  console.error(`[${new Date().toISOString()}] adapter=${cfg.adapter}`);
});
