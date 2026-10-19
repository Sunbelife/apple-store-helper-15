#!/usr/bin/env python3
"""Package a Windows x64 portable Wails app with icon and GUI manifest."""
import argparse,hashlib,os,re,subprocess,tempfile,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
VERSION=re.search(r'AppVersion\s*=\s*"([^"]+)"',(ROOT/'desktop/version.go').read_text()).group(1)
def run(args,**kwargs):subprocess.run(args,cwd=ROOT,check=True,**kwargs)
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=ROOT/'dist');args=parser.parse_args()
 out=args.output.resolve();out.mkdir(parents=True,exist_ok=True)
 run(['npm','--prefix','desktop/frontend','ci','--no-audit','--no-fund']);run(['npm','--prefix','desktop/frontend','run','build'])
 resource=ROOT/'desktop/rsrc_windows_amd64.syso'
 if resource.exists():raise RuntimeError('Existing resource file; refusing to overwrite')
 try:
  run(['go','run','github.com/tc-hib/go-winres@v0.3.3','simply','--arch','amd64','--out',str(ROOT/'desktop/rsrc'),'--manifest','gui','--icon',str(ROOT/'desktop/branding/store-logo.png'),'--product-version',VERSION,'--file-version',VERSION,'--product-name','老孙抢你妹 2026','--file-description','Apple Store 门店取货监控','--original-filename','laosun-qiangnimei-2026.exe'])
  with tempfile.TemporaryDirectory(prefix='qiangnimei-windows-') as tmp:
   executable=Path(tmp)/'laosun-qiangnimei-2026.exe'
   run(['go','build','-tags','desktop,production','-trimpath','-ldflags=-s -w -H windowsgui','-o',str(executable),'./desktop'],env=dict(os.environ,GOOS='windows',GOARCH='amd64',CGO_ENABLED='0'))
   raw=executable.read_bytes()
   assert raw[:2]==b'MZ' and b'PE\x00\x00' in raw[:1024], 'Invalid PE executable'
   archive=out/f'laosun-qiangnimei-2026-v{VERSION}-windows-amd64.zip'
   with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
    z.write(executable,executable.name)
    z.writestr('使用说明.txt',f'老孙抢你妹 2026 v{VERSION} Windows x64 测试版\r\n\r\n支持 Windows 10/11 64 位，需要 Microsoft Edge WebView2 Runtime。\r\n解压到可写目录后双击 exe 运行，无需管理员权限。\r\n若系统缺少 WebView2，请从微软官网下载：https://developer.microsoft.com/microsoft-edge/webview2/\r\n\r\n1. 先在默认浏览器的苹果官网将目标产品加入购物袋。\r\n2. 选择地区、型号、颜色、容量和门店，添加后开始监控。\r\n3. 有货时响铃并打开购物袋，手动选择取货门店和付款。\r\n4. 保持联网、避免休眠；风控冷却时等待自动重试。\r\n\r\nWindows 首版未经实机验证，暂不支持应用内安装更新，请到官网重新下载。\r\n此版本未签署 Windows 代码签名。\r\n官网：https://sunbelife.com/projects/qiangnimei/\r\n反馈：https://weibo.com/x1nyang\r\n')
   (out/(archive.name+'.sha256')).write_text(hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name+'\n')
   print(archive)
 finally:
  resource.unlink(missing_ok=True)
if __name__=='__main__':main()
