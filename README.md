# PHP Composer Distribution Cloud Native Buildpack

This buildpack runs the `composer install` command [composer](https://getcomposer.org/) to download project dependencies.
It requires both `composer` and `php` on the path (see [requires](#requires)).

A usage example can be found in the
[`samples` repository under the `php/composer` directory](https://github.com/paketo-buildpacks/samples/tree/main/php/composer).

## Detection

Will add these requires/provisions to the build plan if and only if a `composer.json` file is found.

### Requires:

- `composer`. Will specify version constraints if `BP_COMPOSER_VERSION` is present.
- `php`. Will specify version constraints when these are present:
  1. BP_PHP_VERSION
  2. composer.lock
  3. composer.json

### Provides:

None

## Build

Will run `composer install` in the project workspace to download project dependencies.

## Integration

This CNB currently does not provide anything specific,
so there's no scenario we can imagine where you would need to require it as a dependency.

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

### `$BP_COMPOSER_VERSION`

Use `$BP_COMPOSER_VERSION` to specify a version or range for Composer.
When not specified, the default version will be used, typically the latest. 

Any valid semver range is accepted.

```shell
BP_COMPOSER_VERSION=2.2.*
```

### `$BP_PHP_VERSION`

Use `$BP_PHP_VERSION` to specify a version or range for the PHP requirement.
When specified, it will take priority over the versions specified in either `composer.lock` or `composer.json` files. 
When not specified, the default version will be used, typically the latest.

Any valid semver range is accepted.

```shell
BP_PHP_VERSION=7.4.*
```

### `$BP_COMPOSER_INSTALL_OPTIONS`

Use `$BP_COMPOSER_INSTALL_OPTIONS` to specify options for the Composer [install command](https://getcomposer.org/doc/03-cli.md#install-i).
This buildpacks will always prepend `--no-progress` to the list of install options.
The default is `--no-dev`.

```shell
# Note that env variables will typically be provided to this buildpack using `pack build -e`
BP_COMPOSER_INSTALL_OPTIONS=--prefer-install --ignore-platform-reqs
# will result in an installation command of `composer install --no-progress --prefer-install --ignore-platform-reqs`
BP_COMPOSER_INSTALL_OPTIONS= # Note that this is set to empty
# will result in an installation command of `composer install --no-progress`
unset BP_COMPOSER_INSTALL_OPTIONS
# will result in an installation command of `composer install --no-progress --no-dev`
```

### `$COMPOSER`

The `$COMPOSER` variable allows you to specify the filename of `composer.json`.
When set, this buildpack will use this location instead of `composer.json` in the detection phase. 
This value must be relative to the project root. 

For more information, please reference the [composer docs](https://getcomposer.org/doc/03-cli.md#composer).

```shell
COMPOSER=./somewhere/composer-other.json
```

### `COMPOSER_VENDOR_DIR`

## TODO:
- Add layer caching
- Offline caching
- Add SBOM support
- Add "Composer Global" support
- Custom vendor directory via `$COMPOSER_VENDOR_DIR`
- Check `composer.lock` and `composer.json` for appropriate PHP versions
  - Note that Composer does not actually install PHP
  - https://getcomposer.org/doc/01-basic-usage.md#platform-packages
  - I'm not sure that this version is enforced by any part of the toolchain. 
  - The `php-dist` buildpack only provides 64bit PHP, yet the default `php` dependency in Composer is 32bit
  - What happens if `$COMPOSER` is present? Can we expect `composer.lock` to be a sibling of `composer.json`?
  - It's perfectly valid to have a `composer.json`/`composer.lock` with no PHP dependency
  - The PHP-Dist buildpack only seems to provide 64 bit PHP. Is it a problem if this buildpack only asks for 32 bit?
    - This would be true IFF composer.json has a dependency on `php` and not `php-64bit`
