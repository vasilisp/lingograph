# lingograph

`lingograph` is a Go library for building LLM pipelines. It provides a
flexible, composable way to create complex conversation flows using pipeline
combinators.

## Core Concepts

`lingograph` is built around two main concepts:

### Actor

An **actor** is a component that processes conversation history and generates
responses. Actors can be:

- OpenAI LLM invocations with system prompts
- Custom implementations with specialized behavior

OpenAI-based actors can seamlessly invoke Go functions. These functions receive
structured Go data directly—no need to manually parse JSON or other formats.
This is made possible through the OpenAI Functions API, combined with Go
reflection to minimize boilerplate.

### Pipeline

A **pipeline** represents the overall structure for processing history and
generating responses. Pipelines are built from actors and composed using
combinators:

- **Chain**: executes a sequence of steps in order
- **Parallel**: runs multiple pipelines concurrently
- **Loop**: repeats a pipeline with optional limits

## Quick Start

With a working Go installation, run the following command in your project
directory:

```bash
go get github.com/vasilisp/lingograph
```

Then explore the [`examples`](https://github.com/vasilisp/lingograph/tree/main/examples) directory to see how the core concepts
fit together.
