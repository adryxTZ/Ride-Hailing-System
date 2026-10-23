#!/usr/bin/env python3
"""Generate the service-tile icon set with the OpenAI Images API.

Style reference: DOC/designs/taxi story/1.jpeg (realistic 3D product-render icons on
plain tiles). Colours are brand-neutral: white/neutral bodies with the brand accent
colour passed in, so a rebrand only needs a re-run with another hex.

Usage:  python3 apps/tools/gen-icons.py --out DIR [--accent '#5003C0'] [--only taxi,sand]
Reads OPENAI_API_KEY from apps/tools/.env (gitignored) or the environment.
"""
import argparse
import base64
import json
import os
import sys
import urllib.request
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))

# key -> subject. Keys match the catalogue `icon` names (services) and option values.
SUBJECTS = {
    # services (seed/catalog/*.json → "icon")
    "taxi": "a modern compact sedan taxi car with a small roof sign, three-quarter front view",
    "truck": "a medium box lorry (cargo truck) with a closed cargo body, three-quarter front view",
    "hard-hat": "a yellow construction hard hat resting beside a small neat pile of sand and two bricks",
    "package-variant-closed": "a closed cardboard parcel box with a shipping label and tape",
    "hamburger": "a burger with fries in a paper sleeve",
    "silverware-fork-knife": "a takeaway food bag with a lidded meal container beside it",
    "cart": "a shopping basket filled with fresh vegetables and groceries",
    "wrench": "a crossed wrench and spanner with a small gear",
    "tow-truck": "a tow truck lifting a small hatchback car by its front wheels, three-quarter view",
    "water-pump": "a septic vacuum tanker truck with a coiled suction hose on the side, three-quarter view",
    # construction material options (request schema x-ui icon-grid)
    "water": "a blue plastic water tank with a tap",
    "shovel": "a neat cone-shaped pile of yellow builder's sand with a shovel stuck in it",
    "sack": "a sealed paper bag of cement, slightly leaning",
    "dots-hexagon": "a heap of grey crushed gravel stones",
    "wall": "a stack of rough grey quarry stones",
    "cube-outline": "a short stack of grey concrete hollow blocks",
    "chart-timeline-variant": "a bundle of steel reinforcement bars tied with wire",
    "pine-tree": "a stack of fresh cut timber planks",
}

STYLE = (
    "Realistic 3D product render app icon of {subject}. Photoreal CGI, clean studio lighting from the top left, "
    "soft matte materials, slight isometric three-quarter angle, object fully centred and filling about 80% of the frame. "
    "Colour: mostly white, light grey and natural material colours, with crisp accents in vivid violet {accent} "
    "(trim, stripes, roof sign, labels, handles). No text, no letters, no logos, no people, no background, no ground plane, "
    "no frame; a very faint soft contact shadow directly under the object only. Transparent background."
)


def load_key() -> str:
    if os.environ.get("OPENAI_API_KEY"):
        return os.environ["OPENAI_API_KEY"]
    env = os.path.join(HERE, ".env")
    if os.path.exists(env):
        for line in open(env):
            if line.startswith("OPENAI_API_KEY="):
                return line.split("=", 1)[1].strip()
    sys.exit("OPENAI_API_KEY not set (apps/tools/.env)")


def generate(key: str, model: str, prompt: str, size: str, quality: str) -> bytes:
    body = json.dumps({
        "model": model, "prompt": prompt, "n": 1, "size": size, "quality": quality,
        "background": "transparent", "output_format": "png",
    }).encode()
    req = urllib.request.Request(
        "https://api.openai.com/v1/images/generations", data=body,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=300) as r:
        data = json.load(r)
    return base64.b64decode(data["data"][0]["b64_json"])


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--accent", default="#5003C0")
    ap.add_argument("--only", default="")
    ap.add_argument("--model", default="gpt-image-1")
    ap.add_argument("--size", default="1024x1024")
    ap.add_argument("--quality", default="medium")
    ap.add_argument("--workers", type=int, default=4)
    a = ap.parse_args()
    key = load_key()
    os.makedirs(a.out, exist_ok=True)
    names = [n for n in a.only.split(",") if n] or list(SUBJECTS)

    def one(name: str) -> str:
        path = os.path.join(a.out, f"{name}.png")
        prompt = STYLE.format(subject=SUBJECTS[name], accent=a.accent)
        try:
            png = generate(key, a.model, prompt, a.size, a.quality)
        except urllib.error.HTTPError as e:  # surface the API's message
            return f"FAIL {name}: {e.code} {e.read().decode()[:300]}"
        except Exception as e:  # noqa: BLE001
            return f"FAIL {name}: {e}"
        open(path, "wb").write(png)
        return f"ok   {name} ({len(png) // 1024} KB)"

    with ThreadPoolExecutor(max_workers=a.workers) as ex:
        for line in ex.map(one, names):
            print(line, flush=True)


if __name__ == "__main__":
    main()
