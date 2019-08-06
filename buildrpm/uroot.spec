%global vendor1     TrenchBoot
%global vendor2     digitalocean

# upstream u-root uses hiphen in name in source tree.
Name:           u-root
Version:        5.0.0	
Release:        0.9%{?dist}
Summary:        u-root initramfs used in UEK secure launch kernel.
Group:		    Unspecified
License:        BSD 3-Clause License 
URL:		    http://u-root.tk/
Source0:        %{name}-%{version}.tar.gz
Source1:        %{vendor1}.tar.gz
Source2:        %{vendor2}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-buildroot
BuildRequires:  golang git

%define initramfs_name uroot-initramfs.cpio
%define gopath %{_builddir}/%{name}-%{version}

%description
u-root("universal root") creates an embeddable root file system intended to be used as initramfs in UEK secure launch kernel.

%prep
%setup -c -n %{name}-%{version}/src/github.com/ -q -T -a 1 -a 2
%setup -c -n %{name}-%{version}/src/github.com/u-root -q -D

%build
cd %{name}
GOPATH=%{gopath} go build -v -o u-root
GOPATH=%{gopath} ./u-root -format=cpio -build=bb -files `which modprobe` -files `which depmod` -files /lib/modules/`uname -r`/ -o %{initramfs_name} ./cmds/boot/* ./cmds/exp/* ./cmds/core/* ./examples/sluinit/*

%install
mkdir -p %{buildroot}/boot
install -m 0755 %{gopath}/src/github.com/u-root/u-root/%{initramfs_name} %{buildroot}/boot/

%files
/boot/%{initramfs_name}

%clean

%changelog
* Mon Aug 5 2019 Simran Singh simran.p.singh@oracle.com
- initial spec file for u-root
