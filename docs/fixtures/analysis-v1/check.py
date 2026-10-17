"""Check curated design fixtures; no HTTP server, simulator or Scala engine.

Only Python's standard library is used. This executable oracle checks the
documented examples and arithmetic; it is not production analysis code.
Run from any directory: python docs/fixtures/analysis-v1/check.py
"""
from collections import Counter
from copy import deepcopy
from datetime import datetime
from decimal import Decimal
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[2]


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, f"duplicate JSON key: {key}")
        value[key] = item
    return value


def load(path):
    return json.loads(path.read_text(encoding="utf-8"),
                      parse_float=Decimal, object_pairs_hook=unique_object,
                      parse_constant=lambda value: require(False, value))


def num(value):
    return isinstance(value, (int, Decimal)) and not isinstance(value, bool)


def exact_keys(obj, required, optional=()):
    require(set(required) <= set(obj) <= set(required) | set(optional),
            f"incorrect fields: {set(obj)}")


def valid_profile(profile):
    exact_keys(profile, ["profileSchemaVersion", "profileVersion", "gameId",
                         "playersField", "capabilities", "specializedAnalyzers"])
    require(type(profile["profileSchemaVersion"]) is int
            and profile["profileSchemaVersion"] == 1, "profile schema version")
    require(re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", profile["profileVersion"]), "profile version")
    require(profile["gameId"] and profile["playersField"] == "/data/players", "profile identity")
    instances, output_ids = set(), set()

    def reserve(value, collection):
        require(isinstance(value, str) and value.strip() and value not in collection,
                f"duplicate/empty identifier: {value}")
        collection.add(value)

    def result_definition(definition):
        for name in ("id", "label", "scope", "unit"):
            require(name in definition, f"missing {name}")
        reserve(definition["id"], output_ids)
        require(definition["scope"] in ("player", "match"), "scope")
        require(definition["label"] and definition["unit"], "label/unit")

    for cap in profile["capabilities"]:
        reserve(cap["instanceId"], instances)
        require(cap["availability"] in ("live", "final"), "availability")
        template = cap["templateId"]
        if template == "event-metrics/v1":
            exact_keys(cap, ["templateId", "instanceId", "availability", "metrics"])
            require(cap["metrics"], "empty metrics")
            for definition in cap["metrics"]:
                operation = definition["operation"]
                require(operation in ("count", "sum", "avg"), "operation")
                result_definition(definition)
                required = ["id", "label", "eventType", "operation", "scope", "unit"]
                if operation == "count":
                    exact_keys(definition, required)
                else:
                    exact_keys(definition, required + ["field", "numberKind"], ["minimum"])
                    require(re.fullmatch(r"/data/[^/]+(?:/[^/]+)*", definition["field"]), "field")
                    require(definition["numberKind"] in ("integer", "decimal"), "number kind")
                    require("minimum" not in definition or num(definition["minimum"]), "minimum")
                require(bool(definition["eventType"]), "event type")
        elif template == "duration/v1":
            exact_keys(cap, ["templateId", "instanceId", "availability", "parameters"])
            p = cap["parameters"]
            exact_keys(p, ["id", "label", "scope", "startEventType", "endEventType", "unit"])
            result_definition(p)
            require(p["startEventType"] == "MATCH_STARTED" and p["unit"] == "seconds", "duration")
            require(p["scope"] != "match" or p["endEventType"] == "MATCH_FINISHED", "duration end")
        elif template == "temporal-window/v1":
            exact_keys(cap, ["templateId", "instanceId", "availability", "parameters"])
            p = cap["parameters"]
            exact_keys(p, ["id", "label", "scope", "eventType", "size", "windowSeconds", "overlap"])
            reserve(p["id"], output_ids)
            require(p["label"] and p["scope"] == "player" and cap["availability"] == "live", "window scope")
            require(type(p["size"]) is int and p["size"] >= 2, "window size")
            require(num(p["windowSeconds"]) and p["windowSeconds"] >= 0
                    and p["overlap"] == "consume", "window policy")
        else:
            raise AssertionError(f"unknown template: {template}")
    for analyzer in profile["specializedAnalyzers"]:
        exact_keys(analyzer, ["analyzerId", "instanceId", "availability", "parameters"])
        require(analyzer["analyzerId"] == "tic-tac-toe-line/v1"
                and analyzer["availability"] == "live", "unknown analyzer")
        reserve(analyzer["instanceId"], instances)
        exact_keys(analyzer["parameters"], ["resultMetricId", "patternId"])
        for identifier in analyzer["parameters"].values():
            reserve(identifier, output_ids)


def timestamp(event):
    return datetime.fromisoformat(event["timestamp"].replace("Z", "+00:00"))


def seconds(start, end):
    delta = timestamp(end) - timestamp(start)
    return Decimal(delta.days * 86400 + delta.seconds) + Decimal(delta.microseconds) / 1000000


def check_case(case):
    request, expected = load(ROOT / case["request"]), load(ROOT / case["expected"])
    profile = load(ROOT / case["profile"]) if case["profile"] else None
    if profile:
        valid_profile(profile)
        require(profile["gameId"] == request["gameId"], "profile game")
    exact_keys(request, ["gameId", "matchId", "state", "events"])
    exact_keys(expected, ["gameId", "matchId", "state", "metrics", "patterns"])
    for name in ("gameId", "matchId", "state"):
        require(request[name] == expected[name], f"response {name}")
    events = request["events"]
    require(events and events[0]["type"] == "MATCH_STARTED", "start")
    require(request["state"] in ("open", "closed"), "state")
    require(sum(e["type"] == "MATCH_STARTED" for e in events) == 1, "multiple starts")
    require(sum(e["type"] == "MATCH_FINISHED" for e in events)
            == int(request["state"] == "closed"), "finish count")
    require(request["state"] != "closed" or events[-1]["type"] == "MATCH_FINISHED", "finish last")
    require(len({e["eventId"] for e in events}) == len(events), "event IDs")
    for event in events:
        exact_keys(event, ["schemaVersion", "eventId", "timestamp", "gameId",
                           "matchId", "type", "data"], ["playerId", "gameVersion"])
        require(type(event["schemaVersion"]) is int and event["schemaVersion"] == 1, "schema")
        require(type(event["data"]) is dict, "data object")
        for name in ("eventId", "timestamp", "gameId", "matchId", "type"):
            require(isinstance(event[name], str) and event[name].strip(), f"event {name}")
        for name in ("playerId", "gameVersion"):
            require(name not in event or isinstance(event[name], str), name)
        require(event["gameId"] == request["gameId"] and event["matchId"] == request["matchId"], "mixed match")
        require(timestamp(event).utcoffset() is not None, "timezone")

    if not profile:
        require(not expected["metrics"] and not expected["patterns"], "unknown game")
        return
    roster = events[0]["data"]["players"]
    require(roster and len(set(roster)) == len(roster), "roster")
    outputs = {}
    declared = {}
    for cap in profile["capabilities"]:
        definitions = cap.get("metrics", [cap.get("parameters")])
        for definition in definitions:
            declared[definition["id"]] = cap
    for analyzer in profile["specializedAnalyzers"]:
        for identifier in analyzer["parameters"].values():
            declared[identifier] = analyzer
    default_status = "final" if request["state"] == "closed" else "provisional"
    basis = {"eventCount": len(events), "lastEventId": events[-1]["eventId"]}
    for kind in ("metrics", "patterns"):
        rows = expected[kind]
        require(rows == sorted(rows, key=lambda r: (r["id"], r["scope"], r["subjectId"])), "output order")
        for row in rows:
            identity = (row["id"], row["subjectId"])
            require(identity not in outputs and row["id"] in declared, "duplicate/unknown output")
            outputs[identity] = row
            require(row["basis"] == basis, "snapshot basis")
            cap = declared[row["id"]]
            origin_key = "templateId" if "templateId" in cap else "analyzerId"
            require(row["source"] == {origin_key: cap[origin_key], "instanceId": cap["instanceId"],
                                     "profileVersion": profile["profileVersion"]}, "source")
            require(row["scope"] in ("player", "match"), "result scope")
            require(row["subjectId"] in (roster if row["scope"] == "player" else [request["matchId"]]), "subject")
            require(row["status"] in ("provisional", "final", "unavailable", "invalid"), "status")
            if row["status"] in ("unavailable", "invalid"):
                require(row.get("reason") and row.get("value", row.get("count")) is None, "missing reason/null")
            else:
                require(row["status"] == default_status, "live/final status")
            if kind == "patterns":
                require(row["count"] == len(row["occurrences"]), "occurrence count")
                used = set()
                for occurrence in row["occurrences"]:
                    ids = occurrence["eventIds"]
                    require(len(ids) == len(set(ids)) and not used.intersection(ids), "overlap")
                    used.update(ids)
                    selected = [next(e for e in events if e["eventId"] == identifier) for identifier in ids]
                    require(selected == sorted(selected, key=lambda e: (timestamp(e), events.index(e))), "occurrence order")
                    require(occurrence["startTimestamp"] == selected[0]["timestamp"]
                            and occurrence["endTimestamp"] == selected[-1]["timestamp"], "occurrence times")

    checked = set()

    def metric(identifier, subject, value, status=None, reason=None):
        row = outputs[(identifier, subject)]
        require(row["value"] == value and row["status"] == (status or default_status),
                f"{case['id']} {identifier}/{subject}: {row}")
        if reason:
            require(row["reason"] == reason, f"reason for {identifier}")
        checked.add((identifier, subject))

    def pattern(identifier, subject, groups):
        row = outputs[(identifier, subject)]
        require([o["eventIds"] for o in row["occurrences"]] == groups, f"pattern {identifier}")
        checked.add((identifier, subject))

    finish = next((e for e in events if e["type"] == "MATCH_FINISHED"), None)
    if finish:
        metric("duration.match", request["matchId"], seconds(events[0], finish))
    else:
        metric("duration.match", request["matchId"], None, "unavailable", "awaiting_finish")

    if request["gameId"] in ("snake", "racing"):
        snake = request["gameId"] == "snake"
        selected_type = "APPLE_EATEN" if snake else "LAP_COMPLETED"
        field = "points" if snake else "durationMs"
        prefix = "apples" if snake else "laps"
        metric(prefix + ".match", request["matchId"], sum(e["type"] == selected_type for e in events))
        for player in roster:
            selected = [e for e in events if e["type"] == selected_type and e.get("playerId") == player]
            metric(prefix + ".player", player, len(selected))
            missing = any(field not in e["data"] for e in selected)
            bad = any(not num(e["data"].get(field)) for e in selected)
            for operation, identifier in (("sum", "points.player" if snake else "lap_time.total.player"),
                                         ("avg", "points_per_apple.player" if snake else "lap_time.avg.player")):
                if missing or bad:
                    metric(identifier, player, None, "invalid", "missing_field" if missing else "invalid_number")
                elif operation == "avg" and not selected:
                    metric(identifier, player, None, "unavailable", "no_samples")
                else:
                    total = sum((Decimal(e["data"][field]) for e in selected), Decimal(0))
                    metric(identifier, player, total if operation == "sum" else total / len(selected))
            if snake:
                deaths = [e for e in events if e["type"] == "PLAYER_DIED" and e.get("playerId") == player]
                metric("deaths.player", player, len(deaths))
                if deaths:
                    require(len(deaths) == 1 and seconds(events[0], deaths[0]) >= 0
                            and (not finish or timestamp(deaths[0]) <= timestamp(finish)), "lifetime")
                    metric("lifetime.player", player, seconds(events[0], deaths[0]))
                else:
                    metric("lifetime.player", player, None, "unavailable",
                           "missing_end" if finish else "awaiting_end")
                groups = {
                    "snake-positive": {"p1": [["a1", "a2", "a3"]]},
                    "snake-tied": {"p1": [["a3", "a1", "a2"]]},
                }.get(case["id"], {}).get(player, [])
                pattern("apples.burst", player, groups)
                for group in groups:
                    samples = [next(e for e in events if e["eventId"] == v) for v in group]
                    require(len(samples) == 3 and seconds(samples[0], samples[-1]) <= 10, "ten-second window")
                if case["id"] == "snake-negative" and player == "p1":
                    require(seconds(selected[0], selected[-1]) == 11, "negative boundary")
    elif request["gameId"] == "tic-tac-toe":
        moves = [e for e in events if e["type"] == "MOVE_PLACED"]
        counts = Counter(e["playerId"] for e in moves)
        for player in roster:
            metric("moves.player", player, counts[player])
        metric("moves.match", request["matchId"], len(moves))
        cells = {}
        for turn, event in enumerate(moves, 1):
            data = event["data"]
            require(data["turn"] == turn and 0 <= data["row"] < 3 and 0 <= data["column"] < 3, "move")
            position = (data["row"], data["column"])
            require(position not in cells and data["symbol"] == events[0]["data"]["symbols"][event["playerId"]], "board")
            cells[position] = event
        lines = ([[(r, c) for c in range(3)] for r in range(3)]
                 + [[(r, c) for r in range(3)] for c in range(3)]
                 + [[(0, 0), (1, 1), (2, 2)], [(0, 2), (1, 1), (2, 0)]])
        winners = [line for line in lines if all(p in cells for p in line)
                   and len({cells[p]["playerId"] for p in line}) == 1]
        outcome = "win" if winners else "draw" if len(cells) == 9 else "in_progress"
        winner = cells[winners[0][0]]["playerId"] if winners else None
        metric("board.result", request["matchId"], {"outcome": outcome, "winnerId": winner})
        groups = [[cells[p]["eventId"] for p in winners[0]]] if winners else []
        pattern("board.winning_line", request["matchId"], groups)
        if winners:
            details = outputs[("board.winning_line", request["matchId"])]["occurrences"][0]["details"]
            require(details == {"winnerId": winner, "cells": [{"row": r, "column": c} for r, c in winners[0]]}, "winning line")
    else:
        raise AssertionError("oracle only supports the curated fixtures")
    require(checked == set(outputs), f"unchecked outputs: {set(outputs) - checked}")


def check_profile_failures(profile):
    mutations = [
        lambda p: p.update(profileSchemaVersion=2),
        lambda p: p.update(unexpected=True),
        lambda p: p["capabilities"][0].update(templateId="unknown/v1"),
        lambda p: p["capabilities"][1].update(instanceId=p["capabilities"][0]["instanceId"]),
        lambda p: p["capabilities"][0]["metrics"][1].update(id=p["capabilities"][0]["metrics"][0]["id"]),
        lambda p: p["capabilities"][0]["metrics"][2].pop("field"),
        lambda p: p["capabilities"][0]["metrics"][2].update(operation="min"),
        lambda p: p["capabilities"][0]["metrics"][2].update(numberKind="text"),
        lambda p: p["capabilities"][0]["metrics"][2].update(minimum="zero"),
    ]
    for mutation in mutations:
        invalid = deepcopy(profile)
        mutation(invalid)
        try:
            valid_profile(invalid)
        except (AssertionError, KeyError):
            continue
        raise AssertionError("invalid profile accepted")
    return len(mutations)


def main():
    manifest = load(ROOT / "manifest.json")
    require(manifest["fixtureSchemaVersion"] == 1, "fixture version")
    ids = [case["id"] for case in manifest["cases"]]
    require(len(ids) == len(set(ids)) == 13, "fixture inventory")
    json_files = list((REPO / "docs").rglob("*.json"))
    for path in json_files:
        load(path)
    for case in manifest["cases"]:
        require(case["derivation"], "missing derivation")
        check_case(case)
    failures = check_profile_failures(load(REPO / "docs/examples/profiles/racing.json"))
    print(f"PASS: {len(json_files)} JSON files, {len(ids)} fixture pairs, {failures} invalid profile mutations")
    print("This checks design examples, not a running Scala service or production engine.")


if __name__ == "__main__":
    main()
