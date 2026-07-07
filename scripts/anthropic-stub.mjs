#!/usr/bin/env node
// Anthropic Messages API stub: drives Claude Code with no real model, no tokens, no key.
//
// Pointed at by ANTHROPIC_BASE_URL, it makes headless `claude` issue exactly one Bash tool
// call — $PROBE_CMD — then stop, so the probe runs inside Claude Code's real sandbox
// (bubblewrap on Linux, Seatbelt on macOS). It echoes back whatever name Claude Code gives
// its Bash tool and answers everything else trivially, so it survives CLI churn.
//
// Env: PORT (default 8787), PROBE_CMD (command to run), BASH_TIMEOUT_MS (0 = CLI default),
//      STUB_LOG (optional request-log file; always logged to stderr too).
import http from 'node:http';
import { appendFileSync } from 'node:fs';

const PORT = parseInt(process.env.PORT || '8787', 10);
const PROBE_CMD = process.env.PROBE_CMD || '';
const LOG = process.env.STUB_LOG || '';
const BASH_TIMEOUT_MS = parseInt(process.env.BASH_TIMEOUT_MS || '0', 10);

function log(...parts) {
  const line = `[anthropic-stub] ${parts.join(' ')}`;
  console.error(line);
  if (LOG) try { appendFileSync(LOG, line + '\n'); } catch { /* best effort */ }
}

const rid = (prefix) => prefix + Math.random().toString(36).slice(2, 12);

function sse(res, type, extra) {
  res.write(`event: ${type}\ndata: ${JSON.stringify({ type, ...extra })}\n\n`);
}

// Stream one assistant message as Anthropic SSE; `block` is a tool_use or a text block.
function streamMessage(res, { model, block, stopReason }) {
  res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', Connection: 'keep-alive' });
  sse(res, 'message_start', { message: {
    id: rid('msg_stub_'), type: 'message', role: 'assistant', model, content: [],
    stop_reason: null, stop_sequence: null, usage: { input_tokens: 1, output_tokens: 1 },
  } });
  sse(res, 'content_block_start', { index: 0, content_block: block.type === 'tool_use'
    ? { type: 'tool_use', id: block.id, name: block.name, input: {} }
    : { type: 'text', text: '' } });
  sse(res, 'content_block_delta', { index: 0, delta: block.type === 'tool_use'
    ? { type: 'input_json_delta', partial_json: JSON.stringify(block.input) }
    : { type: 'text_delta', text: block.text } });
  sse(res, 'content_block_stop', { index: 0 });
  sse(res, 'message_delta', { delta: { stop_reason: stopReason, stop_sequence: null }, usage: { output_tokens: 1 } });
  sse(res, 'message_stop', {});
  res.end();
}

http.createServer((req, res) => {
  let raw = '';
  req.on('data', (chunk) => { raw += chunk; });
  req.on('end', () => {
    const path = (req.url || '').split('?')[0];
    let body = {};
    try { body = raw ? JSON.parse(raw) : {}; } catch { /* tolerate non-JSON */ }
    const model = body.model || 'claude-sonnet-4-5';

    if (path.endsWith('/count_tokens')) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      return res.end(JSON.stringify({ input_tokens: 1 }));
    }

    if (path.endsWith('/v1/messages')) {
      const bash = (Array.isArray(body.tools) ? body.tools : []).find((t) => t && /^bash$/i.test(t.name || ''));
      const content = (Array.isArray(body.messages) ? body.messages : [])
        .flatMap((m) => (Array.isArray(m.content) ? m.content : []));
      const hasToolResult = content.some((c) => c && c.type === 'tool_result');

      // Surface the probe's output / any sandbox error to the run log.
      for (const tr of content.filter((c) => c && c.type === 'tool_result')) {
        const text = typeof tr.content === 'string' ? tr.content
          : (Array.isArray(tr.content) ? tr.content.map((c) => c && c.text).filter(Boolean).join('') : '');
        if (text) log(`tool_result${tr.is_error ? ' (ERROR)' : ''}: ${text.slice(0, 800).replace(/\n/g, ' \\n ')}`);
      }

      if (bash && !hasToolResult && PROBE_CMD) {
        const input = { command: PROBE_CMD };
        if (BASH_TIMEOUT_MS > 0) input.timeout = BASH_TIMEOUT_MS;
        log(`Bash tool_use (${bash.name}) -> ${PROBE_CMD}`);
        streamMessage(res, { model, stopReason: 'tool_use', block: { type: 'tool_use', id: rid('toolu_stub_'), name: bash.name, input } });
      } else {
        log(`end_turn (bash=${!!bash} toolResult=${hasToolResult})`);
        streamMessage(res, { model, stopReason: 'end_turn', block: { type: 'text', text: 'done' } });
      }
      return;
    }

    // Background/model-probe calls we don't model: answer trivially so the CLI proceeds.
    log(`catch-all ${req.method} ${path}`);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end('{}');
  });
}).listen(PORT, '127.0.0.1', () => log(`listening on 127.0.0.1:${PORT} (PROBE_CMD ${PROBE_CMD ? 'set' : 'MISSING'})`));
