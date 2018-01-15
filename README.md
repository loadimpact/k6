<p align="center"><img src="logo.svg" alt="k6" width="300"></p>

<h3 align="center">Like unit testing, for performance</h3>
<p align="center">A modern load testing tool for developers and testers in the DevOps era.</p>

<p align="center">
  <a href="https://github.com/loadimpact/k6/releases"><img src="https://img.shields.io/github/release/loadimpact/k6.svg" alt="Github release"></a>
  <a href="https://circleci.com/gh/loadimpact/k6/tree/master"><img src="https://img.shields.io/circleci/project/github/loadimpact/k6/master.svg" alt="Build status"></a>
  <a href="https://goreportcard.com/report/github.com/loadimpact/k6"><img src="https://goreportcard.com/badge/github.com/loadimpact/k6" alt="Go Report Card"></a>
  <a href="https://codecov.io/gh/loadimpact/k6"><img src="https://img.shields.io/codecov/c/github/loadimpact/k6/master.svg" alt="Codecov branch"></a>
  <br>
  <a href="https://twitter.com/k6_io"><img src="https://img.shields.io/badge/twitter-@k6_io-55acee.svg" alt="@k6_io on Twitter"></a>
  <a href="https://k6.io/slack"><img src="https://img.shields.io/badge/Slack-k6-ff69b4.svg" alt="Slack channel"></a>
</p>
<p align="center">
	<a href="https://github.com/loadimpact/k6/releases">Download</a> ·
	<a href="https://docs.k6.io">Documentation</a> ·
	<a href="https://k6.io/slack">Community</a>
</p>

---

**k6** is a modern load testing tool, building on [Load Impact](https://loadimpact.com/)'s years of experience in the load and performance testing industry. It provides a clean, approachable scripting API, distributed and cloud execution, with command & control through CLI or a REST API.

This is how load testing should look in the 21st century.

[![](demo.gif)](https://asciinema.org/a/cbohbo6pbkxjwo1k8x0gkl7py)

## Menu

- [Features](#features)
- [Install](#install)
- [Quick Start](#quick-start)
- [Need help or want to contribute?](#need-help-or-want-to-contribute)
- [Development setup](#development-setup)

## Features

- **Scripting in ES6 JS**: support for [modules](https://docs.k6.io/docs/modules) to aid code reusability across an organization
- **Everything as code**: test logic and [configuration options](https://docs.k6.io/docs/options) are both in JS for version control friendliness
- **Automation-friendly**: checks (like asserts) and thresholds
- **HTTP/1.1**, [**HTTP/2**](https://docs.k6.io/docs/http2) and [**WebSocket**](https://docs.k6.io/docs/websockets) protocol support
- **TLS features**: [client certificates](https://docs.k6.io/docs/ssl-tls-client-certificates), [configurable SSL/TLS versions and ciphers](https://docs.k6.io/docs/ssl-tls-version-and-cipher-suites)
- **Batteries included**: [Cookies](https://docs.k6.io/docs/cookies), [Crypto](https://docs.k6.io/docs/k6crypto), [Custom metrics](https://docs.k6.io/docs/result-metrics#section-custom-metrics), [Encodings](https://docs.k6.io/docs/k6encoding), [Environment variables](https://docs.k6.io/docs/environment-variables), JSON, HTML forms and more.
- **Flexible metrics storage**: [InfluxDB](https://docs.k6.io/docs/influxdb-grafana), JSON or [Load Impact Insights](https://docs.k6.io/docs/load-impact-insights)

There's even more! [See all features available in k6.](https://docs.k6.io/welcome)

## Install

### Mac

```bash
brew tap loadimpact/k6
brew install k6
```

### Docker

```bash
docker pull loadimpact/k6
```

### Other Platforms

Grab a prebuilt binary from [the Releases page](https://github.com/loadimpact/k6/releases).

### Build from source
To build from source you need **[Git](https://git-scm.com/downloads)** and **[Go](https://golang.org/doc/install)** (1.8 or newer). Follow these instruction for fast building:

- Get source `go get github.com/loadimpact/k6`
- Now `cd` to `$GOPATH/src/github.com/loadimpact/k6` and run `go build`
- Tada, you can now run k6 using `./k6 run script.js`

Then make sure to put the `k6` binary somewhere in your PATH.

## Quick start

k6 works with the concept of **virtual users** (VUs), which run scripts - they're essentially glorified, parallel `while(true)` loops. Scripts are written using JavaScript, as ES6 modules, which allows you to break larger tests into smaller and more reusable pieces, which makes it easy to scale across an organization.

Scripts must contain, at the very least, a `default` function - this defines the entry point for your VUs, similar to the `main()` function in many other languages:

```js
export default function() {
    // do things here...
}
```

*"Why not just run my script normally, from top to bottom"*, you might ask - the answer is: we do, but code **inside** and **outside** your `default` function can do different things.

Code inside `default` is called "VU code", and is run over and over for as long as the test is running. Code outside of it is called "init code", and is run only once per VU.

VU code can make HTTP requests, emit metrics, and generally do everything you'd expect a load test to do - with a few important exceptions: you can't load anything from your local filesystem, or import any other modules. This all has to be done from init code.

There are two reasons for this. The first is, of course: performance.

If you read a file from disk on every single script iteration, it'd be needlessly slow; even if you cache the contents of the file and any imported modules, it'd mean the *first run* of the script would be much slower than all the others. Worse yet, if you have a script that imports or loads things based on things that can only be known at runtime, you'd get slow iterations thrown in every time you load something new.

But there's another, more interesting reason. By forcing all imports and file reads into the init context, we design for distributed execution. We know which files will be needed, so we distribute only those files. We know which modules will be imported, so we can bundle them up from the get-go. And, tying into the performance point above, the other nodes don't even need writable filesystems - everything can be kept in-memory.

As an added bonus, you can use this to reuse data between iterations (but only for the same VU):

```js
var counter = 0;

export default function() {
    counter++;
}
```

### Running k6

First, create a k6 script to describe what the virtual users should do in your load test:

```js
import http from "k6/http";

export default function() {
  http.get("http://test.loadimpact.com");
};
```

Save it as `script.js`, then run k6 like this:

`k6 run script.js`

(Note that if you use the Docker image, the command is slightly different: `docker run -i loadimpact/k6 run - <script.js`)

For more information on how to get started running k6, please look at the [Running k6](https://docs.k6.io/docs/running-k6) documentation. If you want more info on the scripting API or results output, you'll find that also on [https://docs.k6.io](https://docs.k6.io).

---

## Need help or want to contribute?

Types of questions and where to ask:

- How do I? -- [Stack Overflow](https://stackoverflow.com/questions/tagged/k6) (use tags: k6, javascript, load-testing)
- I got this error, why? -- [Stack Overflow](https://stackoverflow.com/questions/tagged/k6)
- I got this error and I'm sure it's a bug -- [file an issue](https://github.com/loadimpact/k6/issues)
- I have an idea/request -- [file an issue](https://github.com/loadimpact/k6/issues)
- Why do you? -- [Slack](https://k6.io/slack)
- When will you? -- [Slack](https://k6.io/slack)
- I want to contribute/help with development -- [Start here](https://github.com/loadimpact/k6/blob/master/CONTRIBUTING.md), then [Slack](https://k6.io/slack) and [issues](https://github.com/loadimpact/k6/issues)

## Development setup

To get a basic development environment for k6 setup, first make sure you have **[Git](https://git-scm.com/downloads)** and **[Go](https://golang.org/doc/install)** (1.8 or newer) installed and working properly.

Then you can get the k6 source into your Go workspace (`$GOPATH/src`) by running:
```bash
go get github.com/loadimpact/k6
```

**Building from source**:

Now, standing in the repo root (`$GOPATH/src/github.com/loadimpact/k6`) you can build a k6 binary from source by running:
```bash
cd $GOPATH/src/github.com/loadimpact/k6
go build
```

**Running the linter**:

We make use of the [`gometalinter`]() tool to lint the code in CI. To run it locally:
```bash
go get -u github.com/alecthomas/gometalinter
gometalinter --install
gometalinter --deadline 10m --config gometalinter.json --vendor ./...
```

**Running the test suite**:

To exercise the entire test suite:
```bash
go test ./...
```

To run the tests of a specific package:
```bash
go test github.com/loadimpact/k6/core
```

To run just a specific test case use `-run` and pass in a regex that matches the name of the test:
```bash
go test ./.. -run ^TestEngineRun$
```

Combining the two above we can run a specific test case in a specific package:
```bash
go test github.com/loadimpact/k6/core -run ^TestEngineRun$
```

**Contributing code**:

1. First, find an issue that you'd like to work on
2. Fork repo to your Github account
3. Get a development environment setup (get the source into `$GOPATH/src/github.com/YOUR_ACCOUNT/k6` using `go get github.com/YOUR_ACCOUNT/k6`)
4. Write some code
5. Before creating Pull Request please do the following (the CI will check these things when you create the PR so you can save yourself a roundtrip):

    a. Run the Go formatter on the code:
    ```bash
    find . -name '*.go' -exec gofmt -s -w {} +
    ```
    b. Lint the code:
    ```bash
    go get -u github.com/alecthomas/gometalinter
    gometalinter --install
    gometalinter --deadline 10m --config gometalinter.json --vendor ./...
    ```
    c. Vendor check the code (make sure all dependencies exists in `vendor/` folder):
    ```bash
    go get github.com/FiloSottile/vendorcheck
    vendorcheck ./...
    ```
    d. Make sure all tests pass:
    ```bash
    go test ./...
    ```
6. Now you're ready to create the Pull Request!
