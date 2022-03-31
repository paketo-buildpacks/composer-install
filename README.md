# PHP Composer Distribution Cloud Native Buildpack

This buildpack runs the `composer install` command [composer](https://getcomposer.org/) to download project dependencies.
It requires both `composer` and `php` on the path (see [requires](#requires)).

A usage example can be found in the
[`samples` repository under the `php/composer` directory](https://github.com/paketo-buildpacks/samples/tree/main/php/composer).

## Detection

Will add these requires/provisions to the build plan if and only if a `composer.json` file is found.

### Requires:

- `composer`
- `php`

### Provides:

- `composer-packages`

## Build

Will run `composer install` in the project workspace to download project dependencies.
The dependencies will be placed in a new layer and symlinked into the workspace at the 
location specified by `COMPOSER_VENDOR_DIR`, which defaults to `vendor`.

If dependencies are needed for Composer install scripts, use `BP_COMPOSER_INSTALL_GLOBAL`
to specify which dependencies to install. 

## Integration

The PHP Composer CNB provides `composer-packages` as a dependency. Downstream buildpacks
can require that dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]

    # The PHP Composer Install provision is named `composer-packages`.
    # This value is considered part of the public API for the buildpack and will not 
    # change without a plan for deprecation.
    name = "composer-packages"

    # The PHP Composer Install buildpack requires some additional metadata options.
    # If neither metadata.build or metadata.launch is provided, this buidpack will contribute
    # an ignored layer
    [requires.metadata]

        # Setting the build flag to true will ensure that packages installed by running
        # `composer install` are available for subsequent buildpacks during their launch phase
        launch = true

        # Setting the build flag to true will ensure that packages installed by running
        # `composer install` are available for subsequent buildpacks during their build phase
        build = true
```
## Logging Configurations

To configure the level of log output from the **buildpack itself**, set the
`BP_LOG_LEVEL` environment variable at build time either directly or through
a [`project.toml` file](https://github.com/buildpacks/spec/blob/main/extensions/project-descriptor.md)
If no value is set, the default value of `INFO` will be used.

The options for this setting are:
- `INFO`: (Default) log information about the detection and build processes
- `DEBUG`: log debugging information about the detection and build processes

```shell
pack build my-app --env BP_LOG_LEVEL=DEBUG
```

## Usage

To package this buildpack for consumption

```
$ ./scripts/package.sh -v <version>
```

This builds the buildpack's Go source using `GOOS=linux` by default. You can supply another value as the first argument to package.sh.

## Configuration

### `COMPOSER`

The `COMPOSER` variable allows you to specify the filename of `composer.json`.
When set, this buildpack will use this location instead of `composer.json` in the detection phase.
This value must be relative to the project root.

For more information, please reference the [composer docs](https://getcomposer.org/doc/03-cli.md#composer).

```shell
COMPOSER=somewhere/composer-other.json
```

### `BP_COMPOSER_INSTALL_OPTIONS`

Use `BP_COMPOSER_INSTALL_OPTIONS` to specify options for the Composer [install command](https://getcomposer.org/doc/03-cli.md#install-i).
This buildpacks will always prepend `--no-progress` to the list of install options.
The default is `--no-dev`.

Note: `BP_COMPOSER_INSTALL_OPTIONS` will be parsed using the [shellwords library](https://github.com/mattn/go-shellwords).

```shell
# Note that env variables will typically be provided to this buildpack using `pack build -e`
BP_COMPOSER_INSTALL_OPTIONS=--prefer-install --ignore-platform-reqs
# will result in an installation command of `composer install --no-progress --prefer-install --ignore-platform-reqs`
BP_COMPOSER_INSTALL_OPTIONS= # Note that this is set to empty
# will result in an installation command of `composer install --no-progress`
unset BP_COMPOSER_INSTALL_OPTIONS
# will result in an installation command of `composer install --no-progress --no-dev`
```

### Other environment variables

Other environment variables used by Composer may be passed in to configure Composer behavior. 
See the [full list here](https://getcomposer.org/doc/03-cli.md#environment-variables).
A few examples are shown here.

- `COMPOSER_VENDOR_DIR`:
Used to make Composer install the dependencies into a directory other than `vendor`. 
This value must be underneath the project root.

- `COMPOSER_AUTH`:
Used to set up authentication, for example to add a GitHub OAuth token to increase the 
default rate limit.

## TODO:
- Add SBOM support
- Add support for PHP extensions
