import React from 'react';
import LocaleDropdownNavbarItem from '@theme-original/NavbarItem/LocaleDropdownNavbarItem';
import type LocaleDropdownNavbarItemType from '@theme/NavbarItem/LocaleDropdownNavbarItem';
import type {WrapperProps} from '@docusaurus/types';
import {useLocation} from '@docusaurus/router';

type Props = WrapperProps<typeof LocaleDropdownNavbarItemType>;

// Show the locale switcher only on docs pages. The landing is English-only.
export default function LocaleDropdownNavbarItemWrapper(props: Props): React.ReactNode {
  const {pathname} = useLocation();
  const onDocs = pathname.includes('/docs');
  if (!onDocs) {
    return null;
  }
  return <LocaleDropdownNavbarItem {...props} />;
}
