FROM node:22-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates git ripgrep && rm -rf /var/lib/apt/lists/*
RUN npm install -g @openai/codex@0.153.4
USER node
WORKDIR /home/node
COPY --chown=node:node server.py /home/node/server.py
EXPOSE 8080
CMD ["python3", "/home/node/server.py"]
