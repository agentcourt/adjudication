# OpenClaw AAR Plaintiff Trial

Paste this into the OpenClaw that should act as plaintiff lawyer.  Replace `PASTE_TOKEN_HERE` with the bearer token for the AAR MCP server before sending it.

```text
Act as plaintiff lawyer for AAR case arb-1.

MCP server name: aar-arb-1-plaintiff
MCP endpoint: http://172.16.0.15:8001/mcp?case_id=arb-1&role_id=plaintiff
Bearer token: PASTE_TOKEN_HERE

First configure the MCP server:

openclaw mcp set aar-arb-1-plaintiff '{"url":"http://172.16.0.15:8001/mcp?case_id=arb-1&role_id=plaintiff","transport":"streamable-http","headers":{"Authorization":"Bearer PASTE_TOKEN_HERE"}}'

Then use that MCP server to run the assignment.

Call wait_for_opportunity first.  It may wait up to 30 seconds before returning.

If wait_for_opportunity returns state waiting, call wait_for_opportunity again with the returned after_version.

If wait_for_opportunity returns state ready, read the returned prompt, turn, tools, limits, remaining time, attempts remaining, and opportunity id.  Use the available tools to complete exactly that opportunity.  During openings, do not call evidence tools.  During later evidence phases, inspect the visible record and evidence before filing.  Submit the legal act through submit_decision.  If submit_decision succeeds, call wait_for_opportunity again.

If wait_for_opportunity returns state done, stop and report that the case is done.

If wait_for_opportunity returns state error, report the error and stop.

Do not create a cron job.  Do not listen for inbound HTTP.  Do not ask the user for the next turn.  The MCP adapter and AAR court decide when a plaintiff opportunity is ready.
```
