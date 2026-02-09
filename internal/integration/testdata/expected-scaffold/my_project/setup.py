from setuptools import setup, find_packages

setup(
    name="my_project",
    version="0.1.0",
    author="Test Author",
    description="A short description",
    packages=find_packages(),
    python_requires=">=3.8",
    entry_points={
        "console_scripts": [
            "my_project=my_project.main:main",
        ],
    },
)
