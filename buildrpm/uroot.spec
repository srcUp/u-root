# upstream u-root uses hiphen in name in source tree.
Name:           u-root
Version:        6.0.0   
Release:        7%{?dist}
Summary:        u-root initramfs used in UEK secure launch kernel.
Group:          Unspecified
License:        BSD 3-Clause License 
URL:            http://u-root.tk/
Source0:        %{name}-%{version}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-buildroot
BuildRequires:  golang git iscsi-initiator-utils

%define initramfs_name uroot-initramfs.cpio
%define gopath    %{_builddir}/%{name}-%{version}
%define uroot_top %{gopath}/src/github.com/u-root/u-root
%description
u-root("universal root") creates an embeddable root file system intended to be used as initramfs in UEK secure launch kernel.

%prep
%setup -c -n %{name}-%{version}/src/github.com/u-root -q -D

%build
mv %{name}-%{version} %{name}
cd %{name}
GOPATH=%{gopath} go build -v -o u-root
GOPATH=%{gopath} ./u-root --uinitcmd=sluinit -build=bb -files `which iscsistart` -o %{initramfs_name} ./cmds/exp/* ./cmds/core/*

%install
mkdir -p %{buildroot}/boot
install -m 0755 %{uroot_top}/%{initramfs_name} %{buildroot}/boot/
install -m 0755 %{uroot_top}/securelaunch.policy %{buildroot}/boot/

%files
/boot/%{initramfs_name}
/boot/securelaunch.policy

%clean

%changelog
* Thu Dec 5 2019 Simran Singh simran.p.singh@oracle.com
- after upstream merge changed u-root build line
* Mon Oct 21 2019 Simran Singh simran.p.singh@oracle.com
- removed modules dependency and added kexec iscsistart and cpuid dependency.
- support for tpm1.2 from trenchboot tpmtool and for tpm2.0 from go-tpm
* Wed Aug 7 2019 Simran Singh simran.p.singh@oracle.com
- Created a uroot_top define.
- uroot does not need version in directory name. fix it by mv command
* Mon Aug 5 2019 Simran Singh simran.p.singh@oracle.com
- initial spec file for u-root
