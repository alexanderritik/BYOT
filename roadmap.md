You already have the first primitive of the execution engine working:

upload binary
    ↓
store in MinIO
    ↓
create Test record
    ↓
download binary
    ↓
execute
    ↓
capture output + exit code
    ↓
create TestRun
    ↓
store logs

That's a legitimate starting point for BYOT.

The important thing now is not to throw this away. I'd evolve it into the architecture we discussed.
What you have today

Your current flow is essentially:

HTTP request
     │
     ▼
Run()
     │
     ├── Get Test
     │
     ├── Download binary
     │
     ├── /tmp/test
     │
     ├── runtime.GetRuntime()
     │
     ├── rt.Run()
     │
     ├── collect output
     │
     ├── upload logs
     │
     └── create TestRun

And that's actually a nice separation already:

rt := runtime.GetRuntime(testRes.Runtime)

output, exitCode, err :=
    rt.Run(testRes.UUID, testRes.TimeoutSeconds)

That runtime abstraction is the thing I'd preserve.
The biggest problem right now

Your HTTP handler is currently doing the execution itself.

POST /run
   │
   ▼
HTTP handler
   │
   ▼
download
   │
   ▼
execute
   │
   ▼
wait 30/60/120 seconds
   │
   ▼
respond

That will become painful very quickly.

Imagine 100 tests scheduled around the same minute.

You don't want:

100 HTTP requests
     ↓
100 processes
     ↓
100 containers

inside your API server.

Instead:

POST /run
    │
    ▼
Create Run
    │
    ▼
Queue Job
    │
    ▼
HTTP 202

Then:

                Queue
                  │
       ┌──────────┼──────────┐
       ▼          ▼          ▼
    Worker      Worker     Worker
       │          │          │
       ▼          ▼          ▼
    Runtime     Runtime     Runtime

That's the first major architectural change I'd make.
Your runtime abstraction is exactly the right direction

I'd take what you already have and formalize it.

Something like:

type Runtime interface {
    Prepare(ctx context.Context, test Test) error
    Run(ctx context.Context, test Test) (*Result, error)
    Cleanup(ctx context.Context, test Test) error
}

Then:

Runtime
   │
   ├── GoRuntime
   ├── NodeRuntime
   ├── PlaywrightRuntime
   └── K6Runtime

But here's an important distinction.

Your current:

Go → compile binary → execute binary

is perfectly fine for your current experiment.

I wouldn't make “everything must become a binary” the permanent architecture.
Why?

Playwright is going to expose this immediately.

A Playwright project isn't naturally:

checkout.bin

It's more like:

checkout/
├── package.json
├── package-lock.json
├── playwright.config.ts
├── tests/
│   └── checkout.spec.ts
└── ...

And it needs:

Node
Playwright
Chromium

So eventually your test artifact should be a package, not necessarily a binary.

I'd change the conceptual model from:

Test
 └── Binary

to:

Test
 └── Artifact
      ├── type
      ├── runtime
      ├── command
      └── storage location

Then:

Go
    artifact → binary

Playwright
    artifact → project bundle

k6
    artifact → JS bundle

Python
    artifact → project/package

Much more flexible.
Your current Test model should evolve

Right now you're effectively doing:

Test
├── UUID
├── OriginalFilename
├── Runtime
├── Severity
├── BinaryURL
└── TimeoutSeconds

I'd move toward something like:

type Test struct {
    ID          string
    Name        string

    Runtime     string
    Command     string

    ArtifactKey string

    Timeout     time.Duration

    Schedule    *string

    CreatedAt   time.Time
}

For example:

{
  "name": "checkout",
  "runtime": "playwright",
  "command": "npx playwright test",
  "artifact": "tests/abc123",
  "timeout": 120
}

Or:

{
  "name": "api-health",
  "runtime": "k6",
  "command": "k6 run script.js",
  "artifact": "tests/xyz789",
  "timeout": 60
}

The command is particularly important.

It means your engine doesn't need to understand every possible test framework.

It mostly needs to provide the environment in which the command runs.
Then your runtime becomes an environment builder

This is the architecture I'd aim for:

                    Test
                     │
                     ▼
              Execution Request
                     │
                     ▼
              Runtime Resolver
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
   Playwright       k6           Go
        │            │            │
        ▼            ▼            ▼
   Environment   Environment   Environment
        │            │            │
        └────────────┼────────────┘
                     ▼
                  Runner
                     │
                     ▼
                 Sandbox
                     │
                     ▼
                Command
                     │
                     ▼
                  Result

That's a much stronger abstraction.
The next thing I'd build: Runner

Right now:

rt.Run(...)

is doing too much conceptually.

I'd introduce:

type Runner interface {
    Run(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)
}

Where:

type ExecutionRequest struct {
    RunID       string
    TestID      string
    Runtime     string
    ArtifactKey string
    Command     string
    Timeout     time.Duration
}

And:

type ExecutionResult struct {
    ExitCode  int
    Status    string
    Duration  time.Duration

    Stdout    []byte
    Stderr    []byte

    Artifacts []Artifact
}

Now your API doesn't care how execution happens.
And this is where Docker enters

Your current architecture is effectively:

API process
   │
   └── execute customer binary

I'd move toward:

Worker
   │
   ▼
Docker container
   │
   ├── test artifact
   ├── runtime
   ├── environment
   ├── secrets
   └── command
   │
   ▼
execution

So your Go worker might eventually do something conceptually like:

result, err := runner.Run(ctx, ExecutionRequest{
    RunID:       run.ID,
    Runtime:     test.Runtime,
    ArtifactKey: test.ArtifactKey,
    Command:     test.Command,
    Timeout:     test.Timeout,
})

The runner creates the sandbox.
There's also a subtle bug/security issue in your current code

You're doing:

tmpPath := "/tmp/" + testRes.UUID

That's okay-ish because UUID is controlled by your system, but I'd still avoid manually constructing execution paths like this.

More importantly:

os.Chmod(tmpPath, 0755)

followed by executing the customer's uploaded binary means your API host is executing customer-controlled code directly.

That is the biggest thing I'd change before exposing this to real external users.

For an internal prototype:

    Fine.

For a SaaS:

    Absolutely not.

The customer code should execute inside an isolated sandbox.
Your MinIO choice is actually good

I'd keep this.

You already have:

MinIO
  │
  ├── test artifacts
  └── logs

Eventually:

bucket/
├── tests/
│   └── test-id/
│       └── version-id/
│
└── runs/
    └── run-id/
        ├── stdout.log
        ├── stderr.log
        ├── screenshot.png
        ├── trace.zip
        └── result.json

That's a nice storage model.
One thing I'd change about logs

Right now:

output, exitCode, err := rt.Run(...)

means the entire output lives in memory.

That's okay for your tiny test.

But eventually a test can produce:

10 MB
100 MB
500 MB

You don't want:

[]byte

holding an enormous test output inside the worker.

I'd move toward streaming:

process
   │
   ├── stdout ──────┐
   │                │
   └── stderr ──────┤
                    ▼
              Log collector
                    │
                    ▼
                 MinIO

Then keep only a small tail/summary in the database.
Your TestRun is already heading in the right direction

You have:

TestRun{
    UUID
    TestID
    StartedAt
    DurationMs
    LogURL
    FinishedAt
    Status
    LogSizeBytes
}

I'd eventually add:

Run
├── id
├── test_id
├── status
├── queued_at
├── started_at
├── finished_at
├── duration_ms
├── exit_code
├── error_code
├── worker_id
├── region
├── runtime_version
└── ...

And separately:

RunArtifact
├── run_id
├── type
├── storage_key
└── size

That will support Playwright beautifully later.
Your architecture today vs where I'd take it
Today

                HTTP
                 │
                 ▼
              Handler
                 │
       ┌─────────┼─────────┐
       ▼         ▼         ▼
    Storage    Runtime    DB
                 │
                 ▼
             execute

Next version

                HTTP
                 │
                 ▼
               API
                 │
                 ▼
              Postgres
                 │
                 ▼
                Queue
                 │
                 ▼
               Worker
                 │
                 ▼
              Runner
                 │
                 ▼
             Sandbox
                 │
                 ▼
              Runtime
                 │
                 ▼
              Result
             /      \
            ▼        ▼
         MinIO    Postgres
            │
            ▼
         Artifacts

Eventually

                         BYOT
                          │
              ┌───────────┴───────────┐
              │                       │
          Control Plane          Execution Plane
              │                       │
       API / Scheduler              Queue
       Tests / Alerts                 │
              │             ┌─────────┼─────────┐
              │             ▼         ▼         ▼
              │          Mumbai   Singapore  Frankfurt
              │             │         │         │
              │          Sandbox   Sandbox   Sandbox
              │             │         │         │
              └─────────────┴─────────┴─────────┘
                            │
                         Results
                            │
                 ┌──────────┴──────────┐
                 ▼                     ▼
             Postgres                MinIO
                 │                     │
                 └──────────┬──────────┘
                            ▼
                       Alert Engine

And this is why I like what you've already built

You haven't built the wrong thing.

You've already discovered the core loop:

Artifact
   ↓
Runtime
   ↓
Execute
   ↓
Exit code
   ↓
Logs
   ↓
TestRun

That's the kernel of BYOT.

I'd keep that kernel and build outward rather than rewriting it.

The next three engineering steps I'd personally take are:

    Move execution out of the HTTP handler → queue + worker.

    Put execution inside an isolated container/sandbox.

    Change “binary” into a generic artifact + command model so Playwright/k6 fit naturally.

After those three, I'd implement Playwright as the first real runtime. That's where your current architecture will get stress-tested in a very useful way.
