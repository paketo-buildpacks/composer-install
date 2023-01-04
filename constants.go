package composer

const (
	ComposerPackagesLayerName = "composer-packages"
	ComposerGlobalLayerName   = "composer-global"
	ComposerPhpIniLayerName   = "composer-php-ini"

	// Autoloader Suffix
	ComposerAutoloaderSuffix = "PaketoDefaultAutoloaderSuffix"

	// Dependencies
	ComposerDependency         = "composer"
	ComposerPackagesDependency = "composer-packages"
	PhpDependency              = "php"

	// Files
	DefaultComposerJsonPath = "composer.json"
	DefaultComposerLockPath = "composer.lock"

	// Environment Variables

	// Composer can set the filename for `composer.json` to something else
	// https://getcomposer.org/doc/03-cli.md#composer
	Composer = "COMPOSER"

	// ComposerVendorDir can make Composer install the dependencies into a directory other than `vendor`
	// https://getcomposer.org/doc/03-cli.md#composer-vendor-dir
	ComposerVendorDir = "COMPOSER_VENDOR_DIR"

	// BpComposerInstallGlobal is a space-delimited list of packages to be installed via `composer global require`
	// This is typically so that they will be available during `composer` scripts
	BpComposerInstallGlobal = "BP_COMPOSER_INSTALL_GLOBAL"

	// BpComposerInstallOptions is a list of options to be provided to `composer install`
	// These will be parsed using the shellwords library https://github.com/mattn/go-shellwords
	BpComposerInstallOptions = "BP_COMPOSER_INSTALL_OPTIONS"

	// PhpExtensionDir is the directory containing PHP extensions.
	// It is set by the Paketo buildpack `php-dist`
	PhpExtensionDir = "PHP_EXTENSION_DIR"

	// BpLogLevel can be set to "DEBUG" to show additional log information
	// It will typically be set by a user during the build
	BpLogLevel = "BP_LOG_LEVEL"
)
