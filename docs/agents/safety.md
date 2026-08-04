# Safety and permissions for agents

## Local network

Bind Tapioca to `127.0.0.1` by default. A local server has no API-key security
boundary and must not be exposed to a LAN or the internet without explicit
approval and an external authentication layer.

## Downloads and disk

Models can range from a few gigabytes to tens of gigabytes. Inspect the catalog
and inform the user before initiating a large pull. Never delete models,
adapters, voices, or generated media unless the user asks.

## Tool execution

Treat model-produced tool calls as untrusted proposals. Validate names and
arguments and apply the host agent's normal approval rules. Set an iteration
limit and stop repeated or malformed calls.

## Files

Write outputs only to the requested workspace or a clearly reported default.
Do not send local prompts, files, voices, or generations to a remote service
unless the user explicitly requested that service.

## Voices and likeness

Require permission to clone or imitate a voice. Do not infer consent from
possession of an audio file.

## Agent profiles

Use `tapioca launch` rather than rewriting the user's primary coding-agent
configuration. Tapioca's launch profiles are isolated and recoverable.
