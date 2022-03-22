# PHP Composer Distribution Cloud Native Buildpack

This buildpack provides the [composer](https://getcomposer.org/) dependency by installing the composer binary
onto the build image `$PATH` which makes it available to subsequent buildpacks.
This buildpack will not issue commands to `composer` (such as `composer install`).

A usage example can be found in the
[`samples` repository under the `php/composer` directory](https://github.com/paketo-buildpacks/samples/tree/main/php/composer).

## Detection

Will add these requires/provisions to the build plan if and only if a `composer.json` file is found.

### Requires:

None

### Provides:
- `composer`

## Build

Will install Composer at a location on the `$PATH` of the build or launch image for subsequent buildpacks to use.

## Integration

The PHP Composer CNB provides composer as a dependency. Downstream buildpacks
can require the composer dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]

    # The PHP Composer provision is named `composer`.
    # This value is considered part of the public API for the buildpack and will not 
    # change without a plan for deprecation.
    name = "composer"
    
    # The version of PHP Composer is not required.
    # If not specified the buildpack will provide the default version, which can
    # be seen in the buildpack.toml file.
    # Any valid semver constraint is acceptable.
    version = "0.1.0"

    # The PHP Composer Dist buildpack supports some non-required metadata options.
    [requires.metadata]
    
        # Setting the build flag to true will ensure that Composer
        # is available on the `$PATH` for subsequent buildpacks during their
        # build phase. If you are writing a buildpack that needs to run Composer
        # during its build process, this flag should be set to true.
        build = true
```

## Logging Configurations

To configure the level of log output from the **buildpack itself**, set the
`$BP_LOG_LEVEL` environment variable at build time either directly or through
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
COMPOSER=./somewhere/composer-other.json
```

## TODO:
- Add layer caching
- Offline caching
- Add SBOM support