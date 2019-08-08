%global vendor1     TrenchBoot
%global vendor2     digitalocean
%global module_kernel_version  4.14.35-1923.el7uek.x86_64

# upstream u-root uses hiphen in name in source tree.
Name:           u-root
Version:        5.0.0	
Release:        1.0%{?dist}
Summary:        u-root initramfs used in UEK secure launch kernel.
Group:		    Unspecified
License:        BSD 3-Clause License 
URL:		    http://u-root.tk/
Source0:        %{name}-%{version}.tar.gz
Source1:        %{vendor1}.tar.gz
Source2:        %{vendor2}.tar.gz
Source3:        lib-modules-%{module_kernel_version}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-buildroot
BuildRequires:  golang git kmod

%define initramfs_name uroot-initramfs.cpio
%define gopath    %{_builddir}/%{name}-%{version}
%define uroot_top %{gopath}/src/github.com/u-root/u-root
%define modules   %{gopath}/src/github.com/lib/modules/%{module_kernel_version}
%description
u-root("universal root") creates an embeddable root file system intended to be used as initramfs in UEK secure launch kernel.

%prep
%setup -c -n %{name}-%{version}/src/github.com/ -q -T -a 1 -a 2 -a 3
%setup -c -n %{name}-%{version}/src/github.com/u-root -q -D

%build
mv %{name}-%{version} %{name}
cd %{name}
GOPATH=%{gopath} go build -v -o u-root
GOPATH=%{gopath} ./u-root -format=cpio -build=bb -files `which modprobe` -files `which depmod` -files %{modules} -o %{initramfs_name} ./cmds/boot/* ./cmds/exp/* ./cmds/core/* ./examples/sluinit/*

%install
mkdir -p %{buildroot}/boot
install -m 0755 %{uroot_top}/%{initramfs_name} %{buildroot}/boot/
install -m 0755 %{uroot_top}/buildrpm/securelaunch.policy %{buildroot}/boot/

%files
/boot/%{initramfs_name}
/boot/securelaunch.policy

%clean

%changelog
* Wed Aug 7 2019 Simran Singh simran.p.singh@oracle.com
- Created a uroot_top define.
- uroot does not need version in directory name. fix it by mv command
* Mon Aug 5 2019 Simran Singh simran.p.singh@oracle.com
- initial spec file for u-root
