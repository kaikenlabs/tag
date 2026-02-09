# {{ vars.project_name }}

{{ vars.description }}

## Author

{{ vars.author }}

## License

{% if vars.license != "Apache-2.0" %}
This project is licensed under the {{ vars.license }} license.
{% else %}
Licensed under Apache-2.0. See LICENSE file for details.
{% endif %}

## Version

{{ vars._private_version }}
