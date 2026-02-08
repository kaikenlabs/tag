from setuptools import setup, find_packages

setup(
    name="{{ vars.project_name|lower|replace(' ', '_') }}",
    version="0.1.0",
    author="Test Author",
    description="A short description",
    packages=find_packages(),
    python_requires=">=3.8",
    entry_points={
        "console_scripts": [
            "{{ vars.project_name|lower|replace(' ', '_') }}={{ vars.project_name|lower|replace(' ', '_') }}.main:main",
        ],
    },
)
