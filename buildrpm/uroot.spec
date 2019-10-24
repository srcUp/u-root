%global vendor1     TrenchBoot
%global vendor2     digitalocean
%global vendor3     google

# upstream u-root uses hiphen in name in source tree.
Name:           u-root
Version:        5.0.0	
Release:        1%{?dist}
Summary:        u-root initramfs used in UEK secure launch kernel.
Group:		    Unspecified
License:        BSD 3-Clause License 
URL:		    http://u-root.tk/
Source0:        %{name}-%{version}.tar.gz
Source1:        %{vendor1}.tar.gz
Source2:        %{vendor2}.tar.gz
Source3:        %{vendor3}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-buildroot
BuildRequires:  golang git kexec-tools iscsi-initiator-utils cpuid

%define initramfs_name uroot-initramfs.cpio
%define gopath    %{_builddir}/%{name}-%{version}
%define uroot_top %{gopath}/src/github.com/u-root/u-root
%description
u-root("universal root") creates an embeddable root file system intended to be used as initramfs in UEK secure launch kernel.

%prep
%setup -c -n %{name}-%{version}/src/github.com/ -q -T -a 1 -a 2 -a 3
%setup -c -n %{name}-%{version}/src/github.com/u-root -q -D

%build
mv %{name}-%{version} %{name}
cd %{name}
GOPATH=%{gopath} go build -v -o u-root
GOPATH=%{gopath} ./u-root -format=cpio -build=bb -files `which bash` -files `which cpuid` -files `which kexec` -files `which iscsistart` -o %{initramfs_name} ./cmds/boot/* ./cmds/exp/* ./cmds/core/*

%install
mkdir -p %{buildroot}/boot
install -m 0755 %{uroot_top}/%{initramfs_name} %{buildroot}/boot/
install -m 0755 %{uroot_top}/securelaunch.policy %{buildroot}/boot/

%files
/boot/%{initramfs_name}
/boot/securelaunch.policy

%clean

%changelog
* Mon Oct 21 2019 Simran Singh simran.p.singh@oracle.com
- removed modules dependency and added kexec iscsistart and cpuid dependency.
- support for tpm1.2 from trenchboot tpmtool and for tpm2.0 from go-tpm
* Wed Aug 7 2019 Simran Singh simran.p.singh@oracle.com
- Created a uroot_top define.
- uroot does not need version in directory name. fix it by mv command
* Mon Aug 5 2019 Simran Singh simran.p.singh@oracle.com
- initial spec file for u-root
