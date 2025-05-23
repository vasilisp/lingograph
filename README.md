# lingograph

`lingograph` is a Go library for building LLM pipelines. It provides a
flexible, composable way to create complex conversation flows using pipeline
combinators.

## Core Concepts

`lingograph` is built around the following concepts:

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
- **While**: repeats a pipeline while a predicate over the store holds true
- **If**: conditionally executes one of two pipelines based on a predicate over the store

### Store

The **store** provides a way to maintain state between pipeline steps. It
supports type-safe variables that can be shared across different parts of the
pipeline. Store variables can be modified from within functions called by
actors and are particularly useful with conditional pipelines, allowing
you to branch the execution flow based on runtime conditions.

## Quick Start

With a working Go installation, run the following command in your project
directory:

```bash
go get github.com/vasilisp/lingograph
```

Then explore the [`examples`](https://github.com/vasilisp/lingograph/tree/main/examples) directory to see how the core concepts
fit together.

## Example Projects

The following projects demonstrate `lingograph`:

- [velora](https://github.com/vasilisp/velora): An AI-powered command-line
  workout tracker and coach. It uses `lingograph` to build complex pipelines for
  analyzing workout data and producing training recommendations.

- [wikai](https://github.com/vasilisp/wikai): A Git-powered note-taking app with
  AI capabilities. It builds a RAG system utilizing `lingograph` pipelines with
  heavy usage of the (OpenAI) function interface.
