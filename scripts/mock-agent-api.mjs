#!/usr/bin/env node
// General mock for agent-CLI model APIs: drives Claude Code, Codex, gemini-cli, OpenCode, Goose, Pi,
// gptme and Cline with no real model, no tokens, no key. Pointed at by the CLI's base-URL override,
// it makes the CLI issue exactly one shell tool call — $PROBE_CMD — then stop, so the probe runs
// inside the agent's real sandbox. It routes by request path and speaks five protocols, each
// returning $PROBE_CMD:
//
//   POST /v1/messages                    Anthropic          -> Bash tool_use            (Claude Code)
//   POST /v1beta/.../*:generateContent   Gemini             -> run_shell_command call    (gemini-cli)
//   POST /v1/responses                   OpenAI Responses    -> function_call (shell)     (Codex)
//   POST /v1/chat/completions            OpenAI Chat (SSE/JSON) -> tool_calls            (OpenCode/Goose/Pi/gptme/Cline)
//   POST /api/chat                       Ollama (NDJSON/JSON)   -> tool_calls            (native-Ollama clients)
//
// It echoes whatever shell tool name the request advertises (shaping the argument from that tool's
// own schema) and answers anything else trivially, so it survives CLI churn.
//
// Env: PORT (default 8787), HOST (default 127.0.0.1; 0.0.0.0 to reach it from a container),
//      PROBE_CMD (command to run), BASH_TIMEOUT_MS (0 = CLI default),
//      STUB_LOG (optional request-log file; always logged to stderr too).
import http from 'node:http';
import { appendFileSync } from 'node:fs';

const PORT = parseInt(process.env.PORT || '8787', 10);
const HOST = process.env.HOST || '127.0.0.1';
const PROBE_CMD = process.env.PROBE_CMD || '';
const LOG = process.env.STUB_LOG || '';
const BASH_TIMEOUT_MS = parseInt(process.env.BASH_TIMEOUT_MS || '0', 10);

function log(...parts) {
  const line = `[mock-agent-api] ${parts.join(' ')}`;
  console.error(line);
  if (LOG) try { appendFileSync(LOG, line + '\n'); } catch { /* best effort */ }
}
const rid = (prefix) => prefix + Math.random().toString(36).slice(2, 12);
const clip = (s) => String(s).slice(0, 800).replace(/\n/g, ' \\n ');

function sseHead(res) {
  res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', Connection: 'keep-alive' });
}
const event = (res, type, extra) => res.write(`event: ${type}\ndata: ${JSON.stringify({ type, ...extra })}\n\n`);
const data = (res, obj) => res.write(`data: ${JSON.stringify(obj)}\n\n`);

// ── Anthropic /v1/messages (Claude Code) ────────────────────────────────────
function anthropic(res, body) {
  const model = body.model || 'claude-sonnet-4-5';
  const bash = (Array.isArray(body.tools) ? body.tools : []).find((t) => t && /^bash$/i.test(t.name || ''));
  const content = (Array.isArray(body.messages) ? body.messages : []).flatMap((m) => (Array.isArray(m.content) ? m.content : []));
  const done = content.some((c) => c && c.type === 'tool_result');
  for (const tr of content.filter((c) => c && c.type === 'tool_result')) {
    const text = typeof tr.content === 'string' ? tr.content
      : (Array.isArray(tr.content) ? tr.content.map((c) => c && c.text).filter(Boolean).join('') : '');
    if (text) log(`tool_result${tr.is_error ? ' (ERROR)' : ''}: ${clip(text)}`);
  }
  const block = bash && !done && PROBE_CMD
    ? { type: 'tool_use', id: rid('toolu_stub_'), name: bash.name, input: BASH_TIMEOUT_MS > 0 ? { command: PROBE_CMD, timeout: BASH_TIMEOUT_MS } : { command: PROBE_CMD } }
    : { type: 'text', text: 'done' };
  log(`anthropic ${block.type === 'tool_use' ? `${bash.name} -> ${PROBE_CMD}` : `end_turn (bash=${!!bash} done=${done})`}`);
  sseHead(res);
  event(res, 'message_start', { message: { id: rid('msg_stub_'), type: 'message', role: 'assistant', model, content: [], stop_reason: null, stop_sequence: null, usage: { input_tokens: 1, output_tokens: 1 } } });
  event(res, 'content_block_start', { index: 0, content_block: block.type === 'tool_use' ? { type: 'tool_use', id: block.id, name: block.name, input: {} } : { type: 'text', text: '' } });
  event(res, 'content_block_delta', { index: 0, delta: block.type === 'tool_use' ? { type: 'input_json_delta', partial_json: JSON.stringify(block.input) } : { type: 'text_delta', text: block.text } });
  event(res, 'content_block_stop', { index: 0 });
  event(res, 'message_delta', { delta: { stop_reason: block.type === 'tool_use' ? 'tool_use' : 'end_turn', stop_sequence: null }, usage: { output_tokens: 1 } });
  event(res, 'message_stop', {});
  res.end();
}

// ── Gemini *:streamGenerateContent (gemini-cli) ─────────────────────────────
function gemini(res, body) {
  const parts = (Array.isArray(body.contents) ? body.contents : []).flatMap((c) => (Array.isArray(c.parts) ? c.parts : []));
  const done = parts.some((p) => p && p.functionResponse);
  for (const p of parts.filter((p) => p && p.functionResponse)) log(`functionResponse: ${clip(JSON.stringify(p.functionResponse.response ?? p.functionResponse))}`);
  const part = !done && PROBE_CMD
    ? { functionCall: { name: 'run_shell_command', args: { command: PROBE_CMD } } }
    : { text: 'done' };
  log(`gemini ${part.functionCall ? `run_shell_command -> ${PROBE_CMD}` : `stop (done=${done})`}`);
  sseHead(res);
  data(res, { candidates: [{ content: { role: 'model', parts: [part] }, finishReason: 'STOP', index: 0 }] });
  res.end();
}

// ── OpenAI /v1/responses (Codex) ────────────────────────────────────────────
function openaiResponses(res, body) {
  const input = Array.isArray(body.input) ? body.input : [];
  const outputs = input.filter((i) => i && i.type === 'function_call_output');
  for (const o of outputs) log(`function_call_output: ${clip(typeof o.output === 'string' ? o.output : JSON.stringify(o.output || ''))}`);
  const lastText = outputs.length ? (typeof outputs[outputs.length - 1].output === 'string' ? outputs[outputs.length - 1].output : JSON.stringify(outputs[outputs.length - 1].output || '')) : '';
  const running = /session id[:\s]+(\d+)/i.exec(lastText); // unified-exec left the process backgrounded

  const tools = Array.isArray(body.tools) ? body.tools : [];
  const shell = tools.find((t) => t && /shell|exec|bash|command/i.test(t.name || t.type || '') && !/stdin/i.test(t.name || ''));
  const stdin = tools.find((t) => t && /stdin/i.test(t.name || ''));

  let item;
  if (!outputs.length && shell && PROBE_CMD) {
    // Turn 1: run the probe. Shape args from the tool's own schema (cmd vs command) and ask for a
    // long yield so a multi-minute scan finishes in one call where possible.
    const props = (shell.parameters && shell.parameters.properties) || {};
    const args = { [props.cmd ? 'cmd' : 'command']: PROBE_CMD };
    if (props.timeout_ms) args.timeout_ms = 600000;
    if (props.yield_time_ms) args.yield_time_ms = 600000;
    log(`openai ${shell.name} -> ${PROBE_CMD}`);
    item = { type: 'function_call', call_id: rid('call_'), name: shell.name, arguments: JSON.stringify(args) };
  } else if (running && stdin && !/exited|exit code/i.test(lastText)) {
    // The scan is still running in a background session: poll it (empty stdin) until it exits, so
    // the process isn't killed when the turn ends before the probe has written its report.
    const sid = parseInt(running[1], 10);
    log(`openai poll ${stdin.name} session=${sid}`);
    item = { type: 'function_call', call_id: rid('call_'), name: stdin.name, arguments: JSON.stringify({ session_id: sid, chars: '', yield_time_ms: 600000 }) };
  } else {
    // The probe has run. If the agent exposes a completion tool (trae's task_done), call it so its
    // step loop ends cleanly after one run; otherwise a plain assistant message ends the turn (Codex).
    const done = tools.find((t) => t && /task[_-]?done|complete|finish/i.test(t.name || t.type || ''));
    if (done) {
      log(`openai ${done.name} (task complete)`);
      item = { type: 'function_call', call_id: rid('call_'), name: done.name, arguments: '{}' };
    } else {
      log('openai final');
      item = { type: 'message', role: 'assistant', content: [{ type: 'output_text', text: 'done' }] };
    }
  }
  // Per the OpenAI Responses spec, SSE is only used when stream:true (Codex). Non-streaming clients
  // (trae-agent) omit it and expect a single JSON Response object with an `output` array.
  if (body.stream !== true) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ id: rid('resp_'), object: 'response', created_at: 0, status: 'completed', model: body.model || 'mock-model', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } }));
    return;
  }
  sseHead(res);
  data(res, { type: 'response.created', response: { id: rid('resp_') } });
  data(res, { type: 'response.output_item.done', item });
  data(res, { type: 'response.completed', response: { id: rid('resp_') } });
  res.end();
}

// ── OpenAI /v1/chat/completions (OpenCode, Goose, Pi, gptme, Cline, most stubbable agents) ──
function chatCompletions(res, body) {
  const model = body.model || 'mock-model';
  const shell = (Array.isArray(body.tools) ? body.tools : []).find((t) => t && t.function && /bash|shell|exec|command/i.test(t.function.name || ''));
  const messages = Array.isArray(body.messages) ? body.messages : [];
  const done = messages.some((m) => m && m.role === 'tool');
  for (const m of messages.filter((m) => m && m.role === 'tool')) log(`tool result: ${clip(typeof m.content === 'string' ? m.content : JSON.stringify(m.content || ''))}`);

  let message, finish;
  if (shell && !done && PROBE_CMD) {
    // Shape the arg from the tool's own schema — Cline's run_commands takes a `commands` array,
    // most others take a `command` string — and raise the tool's timeout if it advertises one
    // (opencode's bash defaults to 120s, which a full macOS scan overruns; harmless otherwise).
    const props = (shell.function.parameters && shell.function.parameters.properties) || {};
    const args = props.commands ? { commands: [PROBE_CMD] } : { command: PROBE_CMD };
    if (props.timeout) args.timeout = 600000;
    if (props.timeout_ms) args.timeout_ms = 600000;
    log(`chat ${shell.function.name} -> ${PROBE_CMD}`);
    message = { role: 'assistant', content: null, tool_calls: [{ index: 0, id: rid('call_'), type: 'function', function: { name: shell.function.name, arguments: JSON.stringify(args) } }] };
    finish = 'tool_calls';
  } else {
    log(`chat final (shell=${!!shell} done=${done})`);
    message = { role: 'assistant', content: 'done' };
    finish = 'stop';
  }

  const id = rid('chatcmpl_');
  if (body.stream !== true) { // non-streaming clients (e.g. gptme via the OpenAI SDK omit `stream`)
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ id, object: 'chat.completion', created: 0, model, choices: [{ index: 0, message, finish_reason: finish }], usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 } }));
    return;
  }
  sseHead(res);
  const chunk = (choice) => data(res, { id, object: 'chat.completion.chunk', created: 0, model, choices: [choice] });
  chunk({ index: 0, delta: message, finish_reason: null });
  chunk({ index: 0, delta: {}, finish_reason: finish });
  res.write('data: [DONE]\n\n');
  res.end();
}

// ── Ollama /api/chat (agents pointed at a native Ollama provider) ────────────
// Ollama's own wire format: tool_calls carry an `arguments` OBJECT (not a JSON string), and the
// streaming form is newline-delimited JSON ending with `{done:true}` rather than SSE + [DONE].
function ollamaChat(res, body) {
  const model = body.model || 'mock-model';
  const shell = (Array.isArray(body.tools) ? body.tools : []).find((t) => t && t.function && /bash|shell|exec|command/i.test(t.function.name || ''));
  const messages = Array.isArray(body.messages) ? body.messages : [];
  const done = messages.some((m) => m && m.role === 'tool');
  for (const m of messages.filter((m) => m && m.role === 'tool')) log(`ollama tool result: ${clip(typeof m.content === 'string' ? m.content : JSON.stringify(m.content || ''))}`);

  let message;
  if (shell && !done && PROBE_CMD) {
    const props = (shell.function.parameters && shell.function.parameters.properties) || {};
    const args = props.commands ? { commands: [PROBE_CMD] } : { command: PROBE_CMD };
    log(`ollama ${shell.function.name} -> ${PROBE_CMD}`);
    message = { role: 'assistant', content: '', tool_calls: [{ function: { name: shell.function.name, arguments: args } }] };
  } else {
    log(`ollama final (shell=${!!shell} done=${done})`);
    message = { role: 'assistant', content: 'done' };
  }

  const stamp = '1970-01-01T00:00:00Z';
  if (body.stream === false) { // Ollama defaults stream:true, so only non-stream when explicitly false
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ model, created_at: stamp, message, done: true, done_reason: 'stop' }));
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/x-ndjson' });
  res.write(JSON.stringify({ model, created_at: stamp, message, done: false }) + '\n');
  res.write(JSON.stringify({ model, created_at: stamp, message: { role: 'assistant', content: '' }, done: true, done_reason: 'stop' }) + '\n');
  res.end();
}

http.createServer((req, res) => {
  let raw = '';
  req.on('data', (chunk) => { raw += chunk; });
  req.on('end', () => {
    const path = (req.url || '').split('?')[0];
    let body = {};
    try { body = raw ? JSON.parse(raw) : {}; } catch { /* tolerate non-JSON */ }
    // JSON.parse can return null/array/scalar; the handlers assume an object. Coerce so a stray
    // `null` body can't throw. Wrap dispatch too: one malformed request must never kill the server
    // (and with it every in-flight probe) — respond and move on.
    if (!body || typeof body !== 'object' || Array.isArray(body)) body = {};

    try {
      if (path.endsWith('/v1/messages')) return anthropic(res, body);
      if (/generateContent$/i.test(path)) return gemini(res, body); // :generateContent / :streamGenerateContent
      if (path.endsWith('/v1/responses')) return openaiResponses(res, body);
      if (path.endsWith('/v1/chat/completions')) return chatCompletions(res, body);
      if (path.endsWith('/api/chat')) return ollamaChat(res, body);

      // Trivial answers for endpoints the CLIs probe but we don't model.
      res.writeHead(200, { 'Content-Type': 'application/json' });
      if (path.endsWith('/count_tokens')) return res.end('{"input_tokens":1}');
      if (path.endsWith('/v1/models')) return res.end('{"data":[{"id":"mock-model"}]}');
      log(`catch-all ${req.method} ${path}`);
      res.end('{}');
    } catch (err) {
      log(`handler error ${req.method} ${path}: ${err && err.message}`);
      try { if (!res.headersSent) res.writeHead(500, { 'Content-Type': 'application/json' }); res.end('{}'); } catch { /* already closed */ }
    }
  });
}).listen(PORT, HOST, () => log(`listening on ${HOST}:${PORT} (PROBE_CMD ${PROBE_CMD ? 'set' : 'MISSING'})`));
