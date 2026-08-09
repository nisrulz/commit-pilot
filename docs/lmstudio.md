# LM Studio

LM Studio runs local models and exposes an OpenAI-compatible API. Commit Pilot
uses the `openai_compat` provider for this API.

## Install LM Studio

Download LM Studio from [lmstudio.ai/download](https://lmstudio.ai/download).

LM Studio supports macOS, Windows, and Linux. Check the
[system requirements](https://lmstudio.ai/docs/app/system-requirements) before
installing it.

## Download and load a model

Open LM Studio, search for a model, and download a GGUF quantization. The model
identifier is not fixed by LM Studio. It depends on the model you download and
the loaded variant.

The LFM2.5 GGUF is one option:

`LiquidAI/LFM2.5-8B-A1B-GGUF`

You can also download it with the CLI if `lms` is installed:

```bash
lms get LiquidAI/LFM2.5-8B-A1B-GGUF
```

Load the model in LM Studio before starting the server.

## Start the OpenAI-compatible server

Open the **Developer** tab in LM Studio and start the server. The default
server address is `http://localhost:1234`.

Commit Pilot needs the `/v1` base URL:

`http://localhost:1234/v1`

LM Studio documents the available routes in its
[OpenAI compatibility guide](https://lmstudio.ai/docs/developer/openai-compat).

## Configure Commit Pilot

Add the model ID shown by LM Studio to
`~/.config/commit-pilot/config.yaml`:

```yaml
provider: openai_compat
base_url: http://localhost:1234/v1
model: paste-the-id-from-lm-studio
```

Do not assume that the Hugging Face repository name is the API model ID. Run
`commit-pilot --list-models` and copy the exact ID returned by LM Studio.

## Check the connection

```bash
commit-pilot --doctor
commit-pilot --list-models
```

## Run Commit Pilot

```bash
commit-pilot
```
