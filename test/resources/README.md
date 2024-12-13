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

## RHEL

    # build packages
    fpm -s empty -n test-dep -d test -v 1.0.1 -a noarch -t rpm
  
    # create repo
    cd rhel
    docker run -it $(pwd):/repository --rm fedora:latest
    yum install createrepo_c
    createrepo /repository
    exit

    chown -R <user>:<group> repodata

## opkg
    # build a debian based package, copy it to a file without 
    fpm -s empty -n test-dep -d test -v 1.0.1 -a all -t deb
    cp test-dep_1.0.1_all.deb test-dep_1.0.1.deb 
    bash utils/deb2ipk.sh all test-dep_1.0.1.deb

    # generate repo index
    bash utils/ipkg-make-index.sh . | gzip > Packages.gz


