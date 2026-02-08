# {{ cookiecutter.project_name }}

{{ cookiecutter.description }}

## Author

{{ cookiecutter.author }} <{{ cookiecutter.email }}>

## License

{% if cookiecutter.open_source_license != "None" %}
This project is licensed under the {{ cookiecutter.open_source_license }} license.
{% else %}
This project is proprietary.
{% endif %}

## Version

{{ cookiecutter.version }}
