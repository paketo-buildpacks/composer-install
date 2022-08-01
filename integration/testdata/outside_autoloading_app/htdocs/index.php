<?php

declare(strict_types=1);

require_once __DIR__ . '/../vendor/autoload.php';

if (class_exists('ClassMap')) {
    echo "ClassMap exists";
} else {
    echo "Can't find ClassMap";
}

echo '<br>';

if (class_exists('Application\\NonVendorClass')) {
    echo "NonVendorClass exists";
} else {
    echo "Can't find NonVendorClass";
}
