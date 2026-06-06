# AAR Service And Case API Startup

## Problem

Before this change, `aar service` started `aar case` with role API listeners on ephemeral ports, then parsed child stderr to discover the private Lawyer and Council API URLs.  That design made stderr part of the control protocol.  Logs should remain logs.

Before this change, `aar case` also started separate HTTP listeners for lawyer and council role APIs.  One case owns one arbitration state, so one private HTTP listener can serve every child role API path.  The two-port split added routing state without adding isolation or correctness.

## Target Design

`aar service` assigns one concrete private child API address before it starts `aar case`.  It passes that address to the child as an ordinary command-line argument.  `aar case` listens on that address and serves `/health`, `/lawyerapi/v1/...`, and, when council API mode is active, `/councilapi/v1/...`.

The service records the assigned private base URL in the case record.  After starting the child process, the service starts a startup poll against `GET /health` on that private base.  When the health endpoint returns success, the service marks the case `running`.  If the configured startup timeout expires first, the service marks startup failed with an explicit error.

The service no longer discovers URLs from stderr.  Child stderr remains available in the case log for diagnosis, but it does not carry service control data.

## HTTP Shape

The child case API uses one private base URL:

```text
http://127.0.0.1:PORT
```

The child serves these paths on that base:

```text
GET  /health
GET  /lawyerapi/v1/get
GET  /lawyerapi/v1/wait
GET  /lawyerapi/v1/status
GET  /lawyerapi/v1/result
POST /lawyerapi/v1/do
GET  /councilapi/v1/get
GET  /councilapi/v1/wait
POST /councilapi/v1/do
```

The public service API remains unchanged.  Public lawyer requests still enter through `/lawyerapi/v1/...`, and public council requests still enter through `/councilapi/v1/...`.  Internally, both route groups forward to the same child private base.

## Startup Behavior

The startup sequence is:

1. `aar service` chooses a concrete local child API address.
2. `aar service` starts `aar case --caseapi-addr ADDR`.
3. `aar service` records `caseapi_base = http://ADDR`.
4. `aar service` polls `GET http://ADDR/health`.
5. A successful health response marks the case `running`.
6. Startup timeout marks the case `failed`.

The poll starts immediately after the child process starts.  It is not triggered lazily by the first lawyer or council request.
