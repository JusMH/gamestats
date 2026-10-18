"""Real Go -> Scala boundary checks, invoked by the existing Docker verifier."""
import http.client
import json


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def event(match, identity, kind, data=None):
    return {"schemaVersion": 1, "eventId": identity,
            "timestamp": "2026-10-18T12:00:00Z", "gameId": "snapshot-limits",
            "matchId": match, "type": kind, "data": {} if data is None else data}


def sized_event(match, identity, kind, size):
    value = event(match, identity, kind, {"parts": []})
    remaining = size - len(encode(value))
    parts = value["data"]["parts"]
    while remaining:
        overhead = 3 if parts else 2
        length = min(60000, remaining - overhead)
        if length < 0:
            raise AssertionError("padding is too small")
        tail = remaining - overhead - length
        if 0 < tail < 3:
            length -= 3 - tail
        parts.append("x" * length)
        remaining -= overhead + length
    if len(encode(value)) != size:
        raise AssertionError("incorrect test event size")
    return value


def check_limits(address):
    host, port = address.rsplit(":", 1)
    if host != "127.0.0.1":
        raise AssertionError("expected an isolated localhost service")
    connection = http.client.HTTPConnection(host, int(port), timeout=30)

    def request(method, path, body=None, expected=200):
        raw = None if body is None else encode(body)
        if raw is not None and len(raw) > 1 << 20:
            raise AssertionError("boundary test must respect Go's per-POST limit")
        connection.request(method, path, raw, {"Content-Type": "application/json"})
        response = connection.getresponse()
        data = json.loads(response.read())
        if response.status != expected:
            raise AssertionError(f"{method} {path}: {response.status}, expected {expected}: {data}")
        return data

    def summary(match):
        return request("GET", "/matches/snapshot-limits/" + match)

    def results(match, state):
        actual = request("GET", "/results/snapshot-limits/" + match)
        if actual != {"gameId": "snapshot-limits", "matchId": match,
                      "state": state, "metrics": [], "patterns": []}:
            raise AssertionError("Scala did not analyze the boundary snapshot")

    try:
        match = "event-capacity"
        request("POST", "/events", event(match, "start", "MATCH_STARTED"))
        for i in range(9998):
            request("POST", "/events", event(match, str(i), "CUSTOM"))
        before = summary(match)
        rejected = request("POST", "/events", event(match, "finish", "CUSTOM"), 413)
        if rejected.get("status") != "rejected" or summary(match) != before:
            raise AssertionError("event-cap rejection mutated the match")
        results(match, "open")
        request("POST", "/events", event(match, "finish", "MATCH_FINISHED"))
        if summary(match)["eventCount"] != 10000:
            raise AssertionError("closing slot was not preserved")
        results(match, "closed")

        match = "byte-capacity"
        maximum, reserved = 8 << 20, 1 << 20
        start = event(match, "start", "MATCH_STARTED")
        request("POST", "/events", start)
        envelope = len(encode({"gameId": "snapshot-limits", "matchId": match,
                               "state": "closed", "events": []}))
        used, count = len(encode(start)), 1
        while True:
            available = maximum - reserved - 1 - envelope - used - count
            size = min(900000, available)
            value = sized_event(match, str(count), "CUSTOM", size)
            request("POST", "/events", value)
            count += 1
            used += size
            if size == available:
                break
        before = summary(match)
        request("POST", "/events", event(match, "finish", "CUSTOM"), 413)
        if summary(match) != before:
            raise AssertionError("byte-cap rejection mutated the match")
        results(match, "open")
        closing = sized_event(match, "finish", "MATCH_FINISHED", reserved)
        if envelope + used + len(encode(closing)) + count != maximum:
            raise AssertionError("test snapshot must reach exactly 8 MiB")
        request("POST", "/events", closing)
        results(match, "closed")
        return {"eventBoundary": 10000, "byteBoundary": maximum,
                "openAndClosedAnalyzed": True, "rejectionWithoutMutation": True}
    finally:
        connection.close()
