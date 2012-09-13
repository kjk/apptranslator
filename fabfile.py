import sys, os, os.path, subprocess
import zipfile
from fabric.api import *
from fabric.contrib import *

# Deploys a new version of apptranslator to the server

env.hosts = ['apptranslator.org']
env.user = 'apptranslator'

def git_ensure_clean():
	out = subprocess.check_output(["git", "status", "--porcelain"])
	if len(out) != 0:
		print("won't deploy because repo has uncommitted changes:")
		print(out)
		sys.exit(1)

def git_pull():
	local("git pull")

def git_trunk_sha1():
	# TODO: use "git rev-parse origin" instead?
	return subprocess.check_output(["git", "log", "-1", "--pretty=format:%H"])

def delete_file(p):
	if os.path.exists(p):
		os.remove(p)

def ensure_remote_dir_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)
	#with settings(warn_only=True):
	#	if run("test -d %s" % p).failed:
	#		abort("dir '%s' doesn't exist on remote server" % p)

def ensure_remote_file_exists(p):
	if not files.exists(p):
		abort("dir '%s' doesn't exist on remote server" % p)
	#with settings(warn_only=True):
	#	if run("test -f %s" % p).failed:
	#		abort("file '%s' doesn't exist on remote server" % p	)

def add_dir_files(zip_file, dir):
	for (path, dirs, files) in os.walk(dir):
		for f in files:
			p = os.path.join(path, f)
			zip_file.write(p)

def zip_files(zip_path):
	zf = zipfile.ZipFile(zip_path, mode="w", compression=zipfile.ZIP_DEFLATED)
	blacklist = ["importsumtrans.go"]
	files = [f for f in os.listdir(".") if f.endswith(".go") and not f in blacklist]
	for f in files: zf.write(f)
	zf.write("build.sh")
	zf.write("secrets.json")
	add_dir_files(zf, "ext")
	zf.close()

def deploy():
	if not os.path.exists("secrets.json"): abort("secrets.json doesn't exist locally")
	#git_ensure_clean()
	#git_pull()
	local("./build.sh")
	ensure_remote_dir_exists('www/app')
	ensure_remote_file_exists('www/data/SumatraPDF/translations.dat')
	sha1 = git_trunk_sha1()
	code_path_remote = 'www/app/' + sha1
	if files.exists(code_path_remote):
		abort('code for revision %s already exists on the server' % sha1)
	zip_path = sha1 + ".zip"
	delete_file(zip_path)
	zip_files(zip_path)
	zip_path_remote = 'www/app/' + zip_path
	put(zip_path, zip_path_remote)
	with cd('www/app'):
		run('unzip -q -x %s -d %s' % (zip_path, sha1))
		run('rm -rf %s' % zip_path)
	with cd(code_path_remote):
		run("./build.sh")
	#run('uname -a')

