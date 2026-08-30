# MNIST inference

The shared problem lives here: infer one digit from 28×28 grayscale bytes. The raw dataset and contract are independent of the language used to implement them.

Run `./fetch.sh` once to download the canonical IDX files into `data/`. The Python and Mojo implementations are nested Jades with their own code, environments, models, and metrics.
