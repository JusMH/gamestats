"""Integration fixture replay against independent, real Go and Scala processes.

This is a contract test, not a game simulator. Start Scala and a fresh Go
process first. No profile or expected response is rewritten.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
from decimal import Decimal
import json
from pathlib import Path
import re
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / "docs/fixtures/analysis-v1"


def load(text):
    return json.loads(text, parse_float=Decimal)


def raw_events(text):
    """Send original JSON fragments; never round or re-encode fixture numbers."""
    match = re.search(r'"events"\s*:\s*\[', text)
    assert match, "fixture has no events array"
    decoder = json.JSONDecoder()
    position = match.end()
    while True:
        while text[position].isspace() or text[position] == ",":
            position += 1
        if text[position] == "]":
            return
        _, end = decoder.raw_decode(text, position)
        yield text[position:end].encode("utf-8")
        position = end


def exchange(url, body=None):
    request = Request(url, data=body,
                      headers={"Content-Type": "application/json"})
    with urlopen(request, timeout=10) as response:
        assert response.status == 200, response.status
        return response.read()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--go-url", default="http://127.0.0.1:8080")
    parser.add_argument("--evidence-dir", type=Path)
    args = parser.parse_args()
    if args.evidence_dir:
        args.evidence_dir.mkdir(parents=True, exist_ok=True)
    manifest = load((FIXTURES / "manifest.json").read_text(encoding="utf-8"))
    accepted = {}
    latest = {}
    sequential = 0
    event_count = 0

    def check(case, repeat):
        snapshot = load((FIXTURES / case["request"]).read_text(encoding="utf-8"))
        expected = load((FIXTURES / case["expected"]).read_text(encoding="utf-8"))
        from urllib.parse import quote
        route = "/results/" + quote(snapshot["gameId"], safe="") + "/" + quote(snapshot["matchId"], safe="")
        actual = exchange(args.go_url.rstrip("/") + route)
        assert load(actual) == expected, "complete response differs: " + case["id"]
        if args.evidence_dir:
            (args.evidence_dir / (case["id"] + "." + repeat + ".json")).write_bytes(actual)

    for case in manifest["cases"]:
        text = (FIXTURES / case["request"]).read_text(encoding="utf-8")
        snapshot = load(text)
        key = (snapshot["gameId"], snapshot["matchId"])
        previous = accepted.get(key, [])
        assert snapshot["events"][:len(previous)] == previous, "snapshots must grow by prefixes"
        bodies = list(raw_events(text))
        assert len(bodies) == len(snapshot["events"])
        for event, body in zip(snapshot["events"][len(previous):], bodies[len(previous):]):
            result = load(exchange(args.go_url.rstrip("/") + "/events", body))
            assert result == {"status": "accepted", "eventId": event["eventId"]}
            event_count += 1
        accepted[key] = snapshot["events"]
        latest[key] = case
        check(case, "first")
        check(case, "repeat")
        sequential += 2
    # Earlier racing prefixes no longer exist after closure; compare the latest
    # snapshot of each actual match, never pretend the live prefix is still current.
    parallel_cases = list(latest.values()) * 2
    with ThreadPoolExecutor(max_workers=8) as executor:
        list(executor.map(lambda pair: check(pair[1], "parallel-" + str(pair[0])),
                          enumerate(parallel_cases)))
    summary = {"fixtures": len(manifest["cases"]), "matches": len(latest),
               "events_accepted": event_count, "sequential_responses": sequential,
               "concurrent_responses": len(parallel_cases)}
    if args.evidence_dir:
        (args.evidence_dir / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print("PASS: real Go -> Scala integration: " + json.dumps(summary, sort_keys=True))


if __name__ == "__main__":
    main()
