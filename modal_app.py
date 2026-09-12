"""Modal deployment entrypoint for the visa status monitor."""

import modal


CONTAINER_IMAGE = "ghcr.io/jonathan523/korea-visa-monitor:latest"
STATE_MOUNT = "/state"

image = modal.Image.from_registry(CONTAINER_IMAGE)

# Values in the local .env file are encrypted and injected into the Function.
config = modal.Secret.from_dotenv(__file__)

# This is created automatically on the first deployment and survives restarts.
state_volume = modal.Volume.from_name("krvisa-state", create_if_missing=True)

app = modal.App("krvisa-monitor")


@app.function(
    image=image,
    secrets=[config],
    volumes={STATE_MOUNT: state_volume},
    schedule=modal.Cron("*/10 8-19 * * *", timezone="Asia/Shanghai"),
    cpu=0.125,
    memory=128,
    single_use_containers=True,
    timeout=120,
)
def check_visa():
    """Check once every ten minutes between 08:00 and 20:00 UTC+8."""
    import os

    # Modal Volume is the zero-configuration default. Explicit S3/Upstash
    # settings in .env still take precedence when users choose those backends.
    os.environ.setdefault("VISA_STATE_STORAGE", "local")
    os.environ.setdefault("VISA_STATE_FILE", f"{STATE_MOUNT}/visa_state.json")

    import monitor

    exit_code = monitor._run_main()
    state_volume.commit()

    if exit_code:
        raise RuntimeError("Visa status check failed; see the logs above")


@app.local_entrypoint()
def main():
    """Run one check manually with `modal run modal_app.py`."""
    check_visa.remote()
