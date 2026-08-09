# Unsloth Studio

Unsloth Studio runs local models and exposes an authenticated
OpenAI-compatible API. Commit Pilot uses the `openai_compat` provider for this
API.

## Install and start Unsloth Studio

Install Unsloth Studio with the official installer:

```bash
curl -fsSL https://unsloth.ai/install.sh | sh
```

For Windows, use the PowerShell installer from the
[Unsloth installation guide](https://unsloth.ai/docs/new/studio/install.md).

Start the local server:

```bash
unsloth studio -p 8888
```

Open `http://127.0.0.1:8888` in a browser. Create the initial password if the
installation asks for one.

## Load a model

Open **New Chat** in Unsloth Studio and load a model. You can search for a
Hugging Face model or select a local model. The API model ID depends on the
loaded model and quantization.

The LFM2.5 GGUF is one option. Search for this repository in Unsloth Studio:

`LiquidAI/LFM2.5-8B-A1B-GGUF`

The model ID returned by the API can include a quantization or loaded-model
suffix. Use the value returned by your instance instead of copying the
repository name into the config.

## Create an API key

Open **Settings**, then **API**. Create a key and copy it immediately. Unsloth
shows the key only once.

Export it for Commit Pilot:

```bash
export COMMIT_PILOT_OPENAI_COMPAT_API_KEY=sk-unsloth-...
```

Keep this key private. Anyone who can use it can send requests to the loaded
model.

## Configure Commit Pilot

Unsloth normally serves on port `8888`. Add the exact model ID returned by
`GET /v1/models` to `~/.config/commit-pilot/config.yaml`:

```yaml
provider: openai_compat
base_url: http://localhost:8888/v1
model: paste-the-id-from-unsloth
```

List the models exposed by Unsloth:

```bash
curl http://localhost:8888/v1/models \
  -H "Authorization: Bearer $COMMIT_PILOT_OPENAI_COMPAT_API_KEY"
```

Copy the `id` value from the response into the `model` setting.

## Check the connection

```bash
commit-pilot --doctor
commit-pilot --list-models
```

## Run Commit Pilot

```bash
commit-pilot
```

Read the [Unsloth API guide](https://unsloth.ai/docs/basics/api.md) for API
routes, authentication, model loading, and server security.
