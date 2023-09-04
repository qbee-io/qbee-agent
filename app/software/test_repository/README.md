# Test Repositories

To make test cases a bit more reliable and quick, we cannot depend on external packages or repositories.

This directory will contain repositories which will become part of the test docker images and will be used for testing.

## Debian

Extracting a package (if changes are needed):

    # contents of the package
    dpkg -x pkg.deb ./
    
    # control file
    dpkg -e pkg.deb ./

Building a package:

    dpkg-deb -Zgzip --build <package_dir>

Rebuilding of Packages.gz and Packages.xz (repository index) can be done with the following command:

    dpkg-scanpackages --multiversion . /dev/null | gzip -9c > Packages.gz
    dpkg-scanpackages --multiversion . /dev/null | xz > Packages.xz
