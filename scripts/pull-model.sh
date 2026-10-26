#!/usr/bin/env sh
# Downloads the open-weights model into the optional Ollama container (needs internet once).
set -eu
MODEL="${OLLAMA_MODEL:-hf.co/t-tech/T-lite-it-2.1-GGUF:Q4_K_M}" # T-lite-it-2.1, about 5 GB
# With Ollama on the host (faster on a Mac, see .env.example) run `ollama pull "$MODEL"` there instead.
docker compose --profile ai exec ollama ollama pull "$MODEL"
echo "Model $MODEL is ready. Restart the API with AI_ENABLED=true to use it."
