"""Render Behave Cucumber JSON, including embeddings, as a standalone report."""

import base64
import html
import json
import sys

source, destination = sys.argv[1:3]
with open(source, encoding="utf-8") as report_file:
    features = json.load(report_file)
output = [
    "<!doctype html><meta charset=utf-8><title>TUICast Cucumber</title><h1>TUICast Cucumber</h1>"
]
for feature in features:
    output.append(f"<h2>{html.escape(feature['name'])}</h2>")
    for scenario in feature.get("elements", []):
        output.append(f"<h3>{html.escape(scenario['name'])}</h3><ul>")
        for step in scenario.get("steps", []):
            status = step.get("result", {}).get("status", "unknown")
            output.append(
                f"<li class={status}>{html.escape(step['name'])} — {status}</li>"
            )
            for embedding in step.get("embeddings", []):
                data = base64.b64decode(embedding["data"]).decode("utf-8", "replace")
                output.append(
                    data
                    if embedding["mime_type"] == "text/html"
                    else f"<pre>{html.escape(data)}</pre>"
                )
        output.append("</ul>")
with open(destination, "w", encoding="utf-8") as report_file:
    report_file.write("".join(output))
