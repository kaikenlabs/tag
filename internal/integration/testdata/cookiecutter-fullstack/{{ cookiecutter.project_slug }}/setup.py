from setuptools import setup, find_packages

setup(
    name="{{ cookiecutter.project_slug }}",
    version="{{ cookiecutter._private_version }}",
    author="{{ cookiecutter.author }}",
    description="{{ cookiecutter.description }}",
    packages=find_packages(),
    python_requires=">=3.8",
    entry_points={
        "console_scripts": [
            "{{ cookiecutter.project_slug }}={{ cookiecutter.project_slug }}.main:main",
        ],
    },
)
