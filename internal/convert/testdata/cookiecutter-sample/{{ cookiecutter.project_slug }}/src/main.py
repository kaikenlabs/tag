#!/usr/bin/env python
"""{{ cookiecutter.project_name }} - {{ cookiecutter.description }}"""

__version__ = "{{ cookiecutter.version }}"
__author__ = "{{ cookiecutter.author }}"


def main():
    """Main entry point."""
    print(f"Welcome to {{ cookiecutter.project_name }}!")
    {% if cookiecutter.use_docker %}
    print("Docker support is enabled")
    {% endif %}


if __name__ == "__main__":
    main()
