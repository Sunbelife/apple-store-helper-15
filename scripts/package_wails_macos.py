#!/usr/bin/env python3
"""Build the Wails 2.0 desktop with system WebKit; stage/sign outside File Provider."""
import argparse
import hashlib
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
VERSION = '2.0.6'

def run(args, **kwargs):
    subprocess.run(args, check=True, cwd=ROOT, **kwargs)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, default=ROOT / 'dist')
    parser.add_argument('--arch', choices=('universal', 'arm64', 'amd64'), default='universal')
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=True)
    run(['npm', '--prefix', 'desktop/frontend', 'ci', '--no-audit', '--no-fund'])
    run(['npm', '--prefix', 'desktop/frontend', 'run', 'build'])
    with tempfile.TemporaryDirectory(prefix='qiangnimei2-') as folder:
        stage = Path(folder)
        app = stage / '老孙抢你妹 2026.app'
        mac = app / 'Contents/MacOS'
        resources = app / 'Contents/Resources'
        mac.mkdir(parents=True)
        resources.mkdir(parents=True)
        arches = ('arm64', 'amd64') if args.arch == 'universal' else (args.arch,)
        binaries = []
        for arch in arches:
            binary = stage / arch
            run(['go', 'build', '-tags', 'desktop,production', '-trimpath', '-ldflags=-s -w', '-o', str(binary), './desktop'], env=dict(os.environ, GOOS='darwin', GOARCH=arch, CGO_ENABLED='1'))
            binaries.append(str(binary))
        executable = mac / 'qiangnimei2'
        if len(binaries) == 2:
            run(['lipo', '-create', *binaries, '-output', str(executable)])
        else:
            shutil.copy2(binaries[0], executable)
        iconset = stage / 'AppIcon.iconset'
        iconset.mkdir()
        for size in (16, 32, 128, 256, 512):
            for scale in (1, 2):
                name = f'icon_{size}x{size}' + ('@2x' if scale == 2 else '') + '.png'
                run(['sips', '-z', str(size * scale), str(size * scale), str(ROOT / 'desktop/branding/store-logo.png'), '--out', str(iconset / name)], stdout=subprocess.DEVNULL)
        run(['iconutil', '-c', 'icns', str(iconset), '-o', str(resources / 'AppIcon.icns')])
        plist = dict(CFBundleExecutable='qiangnimei2', CFBundleIdentifier='com.sunbelife.qiangnimei2', CFBundleName='老孙抢你妹 2026', CFBundleDisplayName='老孙抢你妹 2026', CFBundleShortVersionString=VERSION, CFBundleVersion='206', CFBundlePackageType='APPL', CFBundleIconFile='AppIcon', NSHighResolutionCapable=True, LSMinimumSystemVersion='11.0', NSAppTransportSecurity={'NSAllowsLocalNetworking': True})
        (app / 'Contents/Info.plist').write_bytes(plistlib.dumps(plist))
        run(['xattr', '-cr', str(app)])
        run(['codesign', '--force', '--deep', '--sign', '-', str(app)])
        run(['codesign', '--verify', '--deep', '--strict', str(app)])
        archive = out / f'laosun-qiangnimei-2026-v{VERSION}-macos-{args.arch}-local.zip'
        staged = stage / archive.name
        run(['ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', str(app), str(staged)])
        shutil.copyfile(staged, archive)
        (out / (archive.name + '.sha256')).write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + '  ' + archive.name + '\n')
        print(archive)

if __name__ == '__main__':
    main()
