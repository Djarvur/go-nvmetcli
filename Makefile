PKGNAME = nvmetcli
NAME = nvmet
GIT_BRANCH = $$(git branch | grep \* | tr -d \*)
VERSION = $$(basename $$(git describe --tags | tr - . | sed 's/^v//' 2>/dev/null) || echo "0.1.0")
DOCDIR = ./Documentation
GO = go
GOFLAGS = -v
BINARY = nvmetcli
BUILDDIR = build
DISTDIR = dist

all:
	@echo "Usage:"
	@echo
	@echo "  make build        - Builds the Go binary."
	@echo "  make install      - Installs the binary to /usr/sbin."
	@echo "  make test         - Runs Go tests."
	@echo "  make deb          - Builds debian packages."
	@echo "  make rpm          - Builds rpm packages."
	@echo "  make release      - Generates the release tarball."
	@echo "  make doc          - Builds manpages & html docs in ${DOCDIR}."
	@echo
	@echo "  make clean        - Cleanup the local repository build files."
	@echo "  make cleandoc     - Cleanup auto-generated docs in ${DOCDIR}."
	@echo "  make cleanall     - Remove dist/*, build files, auto-gen docs."
	@echo "  make installdoc   - Install man pages (need sudo)."
	@echo "  make uninstalldoc - Uninstall man pages (need sudo)."

build: ${BINARY}

${BINARY}:
	@echo "Building ${BINARY}..."
	@${GO} build ${GOFLAGS} -o ${BINARY} ./cmd/nvmetcli
	@echo "Built ${BINARY}"

install: ${BINARY}
	@echo "Installing ${BINARY} to /usr/sbin..."
	@install -m 755 ${BINARY} /usr/sbin/${BINARY}
	@echo "Installed ${BINARY}"

test:
	@echo "Running Go tests..."
	@${GO} test -v ./internal/nvmet/...

test-coverage:
	@echo "Running Go tests with coverage..."
	@${GO} test -v -coverprofile=coverage.out ./internal/nvmet/...
	@${GO} tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

doc: ${NAME}
	${MAKE} -C ${DOCDIR}

installdoc:
	${MAKE} -C ${DOCDIR} installdoc

uninstalldoc:
	${MAKE} -C ${DOCDIR} uninstalldoc

cleandoc:
	${MAKE} -C ${DOCDIR} clean
	
clean:
	@echo "Cleaning build artifacts..."
	@rm -fv ${BINARY}
	@rm -frv doc
	@rm -frv ${BUILDDIR}
	@rm -fv build-stamp
	@rm -frv results
	@rm -frv ${PKGNAME}-*
	@rm -fv coverage.out coverage.html
	@${GO} clean -cache -testcache -modcache 2>/dev/null || true
	@echo "Finished cleanup."

cleanall: clean cleandoc
	@rm -frv ${DISTDIR}

release: build/release-stamp
build/release-stamp:
	@mkdir -p ${BUILDDIR}
	@echo "Exporting the repository files..."
	@git archive ${GIT_BRANCH} --prefix ${PKGNAME}-${VERSION}/ \
		| (cd ${BUILDDIR}; tar xfp -)
	@echo "Cleaning up the target tree..."
	@rm -f ${BUILDDIR}/${PKGNAME}-${VERSION}/Makefile
	@rm -f ${BUILDDIR}/${PKGNAME}-${VERSION}/.gitignore
	@rm -f ${BUILDDIR}/${PKGNAME}-${VERSION}/${BINARY}
	@echo "Generating debian changelog..."
	@( \
		version=${VERSION}; \
		author=$$(git show HEAD --format="format:%an <%ae>" -s 2>/dev/null || echo "Unknown <unknown@example.com>"); \
		date=$$(git show HEAD --format="format:%aD" -s 2>/dev/null || date -R); \
		day=$$(git show HEAD --format='format:%ai' -s 2>/dev/null \
			| awk '{print $$1}' \
			| awk -F '-' '{print $$3}' | sed 's/^0/ /g' || echo "1"); \
		date=$$(echo $${date} \
			| awk '{print $$1, "'"$${day}"'", $$3, $$4, $$5, $$6}'); \
		hash=$$(git show HEAD --format="format:%H" -s 2>/dev/null || echo "unknown"); \
		echo "${PKGNAME} ($${version}) unstable; urgency=low"; \
		echo; \
		echo "  * Generated from git commit $${hash}."; \
		echo; \
		echo " -- $${author}  $${date}"; \
		echo; \
	) > ${BUILDDIR}/${PKGNAME}-${VERSION}/debian/changelog
	@echo "Generating rpm specfile from template..."
	@cd ${BUILDDIR}/${PKGNAME}-${VERSION}; \
		if [ -d rpm ]; then \
			for spectmpl in rpm/*.spec.tmpl; do \
				if [ -f $${spectmpl} ]; then \
					sed -i "s/Version:.*/Version: ${VERSION}/g" $${spectmpl}; \
					mv $${spectmpl} $$(basename $${spectmpl} .tmpl); \
				fi; \
			done; \
			rmdir rpm 2>/dev/null || true; \
		fi
	@echo "Generating rpm changelog..."
	@if [ -f ${BUILDDIR}/${PKGNAME}-${VERSION}/*.spec ]; then \
		( \
			version=${VERSION}; \
			author=$$(git show HEAD --format="format:%an <%ae>" -s 2>/dev/null || echo "Unknown <unknown@example.com>"); \
			date=$$(git show HEAD --format="format:%ad" -s 2>/dev/null \
				| awk '{print $$1,$$2,$$3,$$5}' || date +"%a %b %d %Y"); \
			hash=$$(git show HEAD --format="format:%H" -s 2>/dev/null || echo "unknown"); \
			echo '* '"$${date} $${author} $${version}-1"; \
			echo "  - Generated from git commit $${hash}."; \
		) >> $$(ls ${BUILDDIR}/${PKGNAME}-${VERSION}/*.spec 2>/dev/null | head -1); \
	fi
	@find ${BUILDDIR}/${PKGNAME}-${VERSION}/ -exec \
		touch -t $$(date -d @$$(git show -s --format="format:%at" 2>/dev/null || echo $$(date +%s)) \
			+"%Y%m%d%H%M.%S" 2>/dev/null || date +"%Y%m%d%H%M.%S") {} \; 2>/dev/null || true
	@mkdir -p ${DISTDIR}
	@cd ${BUILDDIR}; tar -c --owner=0 --group=0 --numeric-owner \
		--format=gnu -b20 --quoting-style=escape \
		-f ../${DISTDIR}/${PKGNAME}-${VERSION}.tar \
		$$(find ${PKGNAME}-${VERSION} -type f | sort)
	@gzip -6 -n ${DISTDIR}/${PKGNAME}-${VERSION}.tar
	@echo "Generated release tarball:"
	@echo "    $$(ls ${DISTDIR}/${PKGNAME}-${VERSION}.tar.gz)"
	@touch ${BUILDDIR}/release-stamp

deb: release build/deb-stamp
build/deb-stamp:
	@echo "Building debian packages..."
	@cd ${BUILDDIR}/${PKGNAME}-${VERSION}; \
		dpkg-buildpackage -rfakeroot -us -uc
	@mv ${BUILDDIR}/*_${VERSION}_*.deb ${DISTDIR}/ 2>/dev/null || true
	@echo "Generated debian packages:"
	@for pkg in $$(ls ${DISTDIR}/*_${VERSION}_*.deb 2>/dev/null); do echo "  $${pkg}"; done
	@touch ${BUILDDIR}/deb-stamp

rpm: release build/rpm-stamp
build/rpm-stamp:
	@echo "Building rpm packages..."
	@mkdir -p ${BUILDDIR}/rpm
	@build=$$(pwd)/${BUILDDIR}/rpm; dist=$$(pwd)/${DISTDIR}/; rpmbuild \
		--define "_topdir $${build}" --define "_sourcedir $${dist}" \
		--define "_rpmdir $${build}" --define "_buildir $${build}" \
		--define "_srcrpmdir $${build}" -ba ${BUILDDIR}/${PKGNAME}-${VERSION}/*.spec
	@mv ${BUILDDIR}/rpm/*-${VERSION}*.src.rpm ${DISTDIR}/ 2>/dev/null || true
	@mv ${BUILDDIR}/rpm/*/*-${VERSION}*.rpm ${DISTDIR}/ 2>/dev/null || true
	@echo "Generated rpm packages:"
	@for pkg in $$(ls ${DISTDIR}/*-${VERSION}*.rpm 2>/dev/null); do echo "  $${pkg}"; done
	@touch ${BUILDDIR}/rpm-stamp

.PHONY: all build install test test-coverage doc installdoc uninstalldoc cleandoc clean cleanall release deb rpm
