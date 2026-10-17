#!/usr/bin/env python3
"""Build a universal macOS app and distributable ZIP."""
import argparse
import hashlib
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tempfile

ROOT=Path(__file__).resolve().parents[1]
def run(args, **kwargs):subprocess.run(args,check=True,cwd=ROOT,**kwargs)
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=ROOT/'dist');args=parser.parse_args()
    out=args.output.resolve();out.mkdir(parents=True,exist_ok=True)
    version=re.search(r'VERSION = "([^"]+)"',(ROOT/'common/const.go').read_text()).group(1)
    stage=tempfile.TemporaryDirectory(prefix='qiangnimei-package-')
    app=Path(stage.name)/'抢你妹18.app';mac=app/'Contents/MacOS';resources=app/'Contents/Resources'
    mac.mkdir(parents=True,exist_ok=True);resources.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='apple-store-build-') as folder:
        tmp=Path(folder);binaries=[]
        for arch in ('arm64','amd64'):
            target=tmp/arch
            run(['go','build','-trimpath','-ldflags=-s -w','-o',str(target),'.'],env=dict(os.environ,GOOS='darwin',GOARCH=arch,CGO_ENABLED='1'))
            binaries.append(str(target))
        run(['lipo','-create',*binaries,'-output',str(mac/'apple-store-helper')])
        iconset=tmp/'AppIcon.iconset';iconset.mkdir()
        for size in (16,32,128,256,512):
            for scale in (1,2):
                name=f'icon_{size}x{size}'+('@2x' if scale==2 else '')+'.png'
                run(['sips','-z',str(size*scale),str(size*scale),str(ROOT/'Icon.png'),'--out',str(iconset/name)],stdout=subprocess.DEVNULL)
        run(['iconutil','-c','icns',str(iconset),'-o',str(resources/'AppIcon.icns')])
    plist={'CFBundleExecutable':'apple-store-helper','CFBundleIdentifier':'com.sunbelife.apple-store-helper','CFBundleName':'抢你妹18','CFBundleDisplayName':'抢你妹18','CFBundleIconFile':'AppIcon','CFBundleShortVersionString':version,'CFBundleVersion':version.replace('.',''),'CFBundlePackageType':'APPL','NSHighResolutionCapable':True,'LSMinimumSystemVersion':'11.0'}
    (app/'Contents/Info.plist').write_bytes(plistlib.dumps(plist))
    run(['xattr','-cr',str(app)])
    run(['codesign','--force','--deep','--sign','-',str(app)])
    run(['codesign','--verify','--deep','--strict',str(app)])
    archive=out/f'qiangnimei18-v{version}-macos-universal.zip'
    run(['ditto','-c','-k','--sequesterRsrc','--keepParent',str(app),str(Path(stage.name)/archive.name)])
    shutil.copyfile(Path(stage.name)/archive.name,archive)
    (out/(archive.name+'.sha256')).write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
    stage.cleanup()
    print(archive)
if __name__=='__main__':main()
