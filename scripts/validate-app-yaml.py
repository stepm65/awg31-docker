#!/usr/bin/env python3
"""Проверка RouterOS /app YAML по схеме tikoci (копия в scripts/routeros-app-yaml.schema.json)."""
import json
import os
import sys

import yaml
from jsonschema.validators import validator_for

here = os.path.dirname(os.path.abspath(__file__))
with open(os.path.join(here, "routeros-app-yaml.schema.json"), encoding="utf-8") as f:
    schema = json.load(f)
cls = validator_for(schema)
cls.check_schema(schema)
validator = cls(schema)

fail = 0
for path in sys.argv[1:]:
    with open(path, encoding="utf-8") as f:
        doc = yaml.safe_load(f)
    errors = [f"{e.message} (в {list(e.absolute_path) or 'корне'})" for e in validator.iter_errors(doc)]
    for name, svc in (doc.get("services") or {}).items():
        for k, v in (svc.get("environment") or {}).items():
            if isinstance(v, bool):
                errors.append(f"environment.{k} в '{name}': bool {v}, нужна строка в кавычках")
    for e in errors:
        print(f"FAIL {path}: {e}")
    if not errors:
        print(f"OK   {path}")
    fail += len(errors)
sys.exit(1 if fail else 0)
