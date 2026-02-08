# {{ cookiecutter.project_name }}

{{ cookiecutter.description }}

## Author

{{ cookiecutter.author }}

## License

{% if cookiecutter.license != "Apache-2.0" %}
This project is licensed under the {{ cookiecutter.license }} license.
{% else %}
Licensed under Apache-2.0. See LICENSE file for details.
{% endif %}

## Version

{{ cookiecutter._private_version }}
