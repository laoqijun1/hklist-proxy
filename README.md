编译：
安装go
用go build proxy.go编译（缺失模块请自己解决）。
ps：因为没在代理里添加清理数据库的功能，特此上传另一个清理数据库的弥补，编译方式参考上面。
ppss：因为是用chatgpt生成，代码可能会惨不忍睹，如果可以请帮忙优化。
使用方法：
取一台vps（选啥看你，不过要仔细斟酌，所有下载流量都会走这台服务器）
输入“chmod 777 文件名”给予文件权限，然后输入“./proxy -port=你自己设的端口 -password='你自己设的密码'”运行代理服务端
最后输入“./clear-sql”定时清理sql数据库
然后在解析网站的后台，解析配置里，把代理下载服务器和密码填进去即可。
![image](https://github.com/user-attachments/assets/07c600c4-3ebe-4ff1-80ee-f3394891fde5)
目前仅发布amd64 linux版本（在阿里云官方的服务器系统上编译的）。
