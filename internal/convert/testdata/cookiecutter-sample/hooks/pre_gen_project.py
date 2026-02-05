#!/usr/bin/env python
"""Pre-generation hook for cookiecutter."""

import sys

# Validate project name
project_name = "{{ cookiecutter.project_name }}"
if not project_name.strip():
    print("ERROR: project_name cannot be empty!")
    sys.exit(1)

print(f"Creating project: {project_name}")
