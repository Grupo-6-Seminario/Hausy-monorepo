#!/usr/bin/env python3
"""Count observed failure phrases by actor. This is a narrow replay, not a judge."""
import json
import pathlib

root = pathlib.Path(__file__).resolve().parents[1]
patterns = {
    "reversed_noise": ["no es ruidoso", "preferencia de tener mucho ruido"],
    "unsupported_guarantee": ["garantiza silencio"],
    "internal_ranking_language": ["el ranking está completo", "perdiendo puntos", "sistema lo clasifica"],
}
report = {
    "premise": "A local model can freely rewrite correct ranking facts without changing their meaning.",
    "limit": "Counts replay only the concrete observed phrases. Manual evidence review establishes semantic failures.",
    "runs": [],
}
for name in ["live-comparison.json", "live-comparison-v2.json"]:
    path = root / "output" / name
    if not path.exists():
        continue
    source = json.loads(path.read_text())
    counts = {actor: {} for actor in ["current_buyer_writer", "free_prototype_writer", "deterministic_fallback"]}
    for case_id, pair in source.get("live", {}).items():
        fallback = next(c["proposed_fallback"] for c in source["cases"] if c["id"] == case_id)
        actors = {
            "current_buyer_writer": pair["baseline"]["content"],
            "free_prototype_writer": pair["proposed"]["content"],
            "deterministic_fallback": fallback,
        }
        for actor, text in actors.items():
            for category, phrases in patterns.items():
                hits = sum(text.lower().count(phrase) for phrase in phrases)
                counts[actor][category] = counts[actor].get(category, 0) + hits
    report["runs"].append({"source": name, "observed_phrase_counts": counts})
print(json.dumps(report, indent=2))
