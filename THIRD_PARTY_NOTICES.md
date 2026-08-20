# Third-party notices

## Bifrost

Alzette links [Maxim Bifrost](https://github.com/maximhq/bifrost) Core v1.7.13
as its buffered provider-protocol engine. Alzette's bounded OpenAI Responses
and Anthropic Messages ingress adapters also adapt Bifrost's hub-and-spoke
mapping and event-sequencing approach.

- Copyright 2025 H3 Labs Inc.
- Licence: Apache License 2.0.
- Upstream licence: https://github.com/maximhq/bifrost/blob/main/LICENSE
- Integration boundary: Bifrost performs buffered provider request/response
  conversion and execution. Alzette retains authentication, tenant/model route
  authority, provider-secret selection, retry policy, cancellation, limits,
  public errors, and logical/per-attempt accounting. Unsupported semantics are
  rejected before execution. The dependency is linked as a Go module and is not
  vendored.

The Apache License 2.0 text is available from the upstream link and at
https://www.apache.org/licenses/LICENSE-2.0.
