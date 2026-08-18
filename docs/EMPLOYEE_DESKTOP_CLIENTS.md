# Use Alzette from Jan or Goose Desktop

This is the shortest employee path to a company model. You never receive or
manage a permanent Alzette API key.

## Before you start

Your company owner must have:

1. invited your exact work email;
2. added you to an Alzette access group; and
3. assigned at least one model endpoint to that group.

Install [Jan Desktop](https://www.jan.ai/docs/desktop/install/linux) or
[Goose Desktop](https://github.com/aaif-goose/goose/releases), and obtain the
`alzette-agent` executable from Alzette. The commands below use the local demo
control URL; a packaged pilot will provide the production URL automatically.

## Jan Desktop

Start Jan through Alzette, using the path where you installed its AppImage:

```bash
ALZETTE_AGENT_ALLOW_INSECURE_LOCAL=true \
  alzette-agent run --control http://127.0.0.1:19081 -- \
  "$HOME/Applications/Jan.AppImage" --appimage-extract-and-run
```

1. Sign in in the browser window that opens.
2. In Jan, open **Settings → Model Providers → +** and choose
   **OpenAI-compatible**.
3. Name the provider `Alzette`. Paste **Jan / OpenAI base URL** and
   **Session key** from the terminal, then save. If `Alzette` already exists,
   open it and replace those two rotating values instead.
4. Select `alzette-chat` (or another model shown by Alzette) and chat.

Jan's official custom-endpoint flow is documented in
[Jan's custom endpoint guide](https://www.jan.ai/docs/desktop/remote-models/custom-endpoint).

## Goose Desktop

If Goose was installed from its Debian package, start it through Alzette with:

```bash
ALZETTE_AGENT_ALLOW_INSECURE_LOCAL=true \
  alzette-agent run --control http://127.0.0.1:19081 -- goose
```

1. Sign in in the browser window that opens.
2. Choose **Connect to a Provider → Add a custom provider → Configure
   manually**. If an `Alzette` provider already exists, edit it instead.
3. Enter these values from the terminal:

   - Provider type: `OpenAI Compatible`
   - Display name: `Alzette`
   - API URL: the printed **Goose API URL**
   - API base path: the printed **Goose API base path**
   - API key: the printed **Session key**
   - Available models: `alzette-chat` (or the model shown by Alzette)
   - Streaming: enabled

4. Create the provider, select the model, and start a chat.

Goose's official custom-provider fields are documented in
[Goose provider setup](https://goose-docs.ai/docs/getting-started/providers/#configure-custom-provider).

## The only rule to remember

Keep the `alzette-agent` terminal open while using the desktop app. Fully quit
the desktop app when finished; Goose remains active when its window is merely
closed, so use **File → Quit** or **Ctrl+Q**. Alzette then revokes the session.

The displayed session key works only against the random loopback address on
your own computer and only while that one `alzette-agent` process is running.
It is not the OAuth token, not the ten-minute `alz_u_` inference credential,
and not a permanent application key. `alzette-agent` keeps those credentials
inside its own memory, remints the short inference credential when needed, and
revokes the grant on exit.

Jan and Goose may retain that now-useless loopback session key in their normal
provider secret store. Goose uses the operating-system keychain when available
and a permission-`0600` `secrets.yaml` fallback otherwise. Neither client
receives or stores the OAuth token or `alz_u_` value. Treat the desktop profile
as private in the same way as any other application profile.

For this local memory-only demo, the loopback URL and session key rotate on
every launch, so update those two provider fields when starting a new session.
Protected durable login, automatic native-client configuration, signed
cross-platform installers, production mail, and remote TLS remain pilot
release work.

## If no model appears

Quit the app and terminal, then ask the company owner to check **Access →
Groups**. Your employee record must be in an enabled group that has an active
model endpoint. Signing in successfully does not grant a model by itself.

## Verified demo scope

The repository's 2026-08-18 local Linux evidence covers packaged Jan Desktop
0.8.4, packaged Goose Desktop 1.46.0, and Pi 0.84.2. Each completed a real
streaming request through the same group-scoped human path, and a full client
exit left zero active human inference tokens. This does not claim other
versions, operating systems, remote production readiness, or broad OpenAI API
compatibility.
