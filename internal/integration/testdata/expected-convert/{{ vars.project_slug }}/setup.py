from setuptools import setup, find_packages

setup(
    name="{{ vars.project_slug }}",
    version="{{ vars._private_version }}",
    author="{{ vars.author }}",
    description="{{ vars.description }}",
    packages=find_packages(),
    python_requires=">=3.8",
    entry_points={
        "console_scripts": [
            "{{ vars.project_slug }}={{ vars.project_slug }}.main:main",
        ],
    },
)
