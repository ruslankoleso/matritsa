#!/usr/bin/env sh
# Downloads the open-weights model into the optional Ollama container (needs internet once).
set -eu
MODEL="${OLLAMA_MODEL:-qwen2.5:3b}"
docker compose --profile ai exec ollama ollama pull "$MODEL"
echo "Model $MODEL is ready. Restart the API with AI_ENABLED=true to use it."
