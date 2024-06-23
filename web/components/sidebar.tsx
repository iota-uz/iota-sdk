'use client';

import Link from 'next/link';
import Image from 'next/image';
import {useState} from 'react';
import * as Icons from '@phosphor-icons/react';
import {Icon} from '@phosphor-icons/react';
import {usePathname} from 'next/navigation';
import {cn} from '@/lib/utils';
import {useTranslations} from "next-intl";

type LinkProps = {
  name: string;
  href?: string;
  children?: LinkProps[];
  icon: Icon;
};

type SidebarLinkProps = {
  name: string;
  icon: Icon;
  isExpanded: boolean;
  active: boolean;
} & LinkProps & React.ComponentProps<'a'>;

export function SidebarLink({
                              className, isExpanded, name, icon: PropIcon, href, active, ...props
                            }: SidebarLinkProps) {
  const linkActiveClass = 'text-white hover:text-gray-100';
  return (
    <Link
      href={href || '#'}
      {...props}
      className={cn(
        'flex items-center gap-4 px-4 py-3 text-base font-medium hover:text-gray-200 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-50',
        active ? linkActiveClass : 'text-gray-400',
        className,
      )}
    >
      <PropIcon className="h-6 w-6"/>
      {name}
      {
        !href && (
          <span className="flex-1"/>
        )
      }
      {
        !href && (
          <Icons.CaretUp
            className={cn('transition-transform duration-300 ease-in-out', isExpanded ? 'rotate-180' : '')}
          />
        )
      }
    </Link>
  );
}

export function SidebarItem(props: LinkProps) {
  const pathname = usePathname();
  const active = pathname == props.href;
  const [isExpanded, setIsExpanded] = useState(false);
  return (
    <li onClick={() => setIsExpanded(!isExpanded)} className={cn('rounded-md', active ? 'bg-primary' : '')}>
      <SidebarLink name={props.name} href={props.href} icon={props.icon} isExpanded={isExpanded} active={active}/>
      {isExpanded && (
        <ul className="space-y-2 pl-4">
          {props.children && props.children.map((link) => (
            <SidebarItem key={link.name} {...link} />
          ))}
        </ul>
      )}
    </li>
  );
}

export function Sidebar() {
  const t = useTranslations('Sidebar');
  const links = [
    {
      name: t('home'),
      icon: Icons.Gauge,
      href: '/',
    },
    {
      name: t('users'),
      icon: Icons.Users,
      href: '/users',
    },
    {
      name: t('bi-chat'),
      icon: Icons.ChatCircle,
      href: '/bi-chat',
    },
    {
      name: t('knowledge-base'),
      icon: Icons.Book,
      href: '/knowledge-base',
    },
    {
      name: t('operations.index'),
      icon: Icons.Pulse,
      children: [
        {
          name: t('operations.employees'),
          icon: Icons.Users,
          href: '/operations/employees',
        },
        {
          name: t('operations.settings'),
          icon: Icons.Gear,
          href: '/settings',
        },
        {
          name: t('operations.calendar'),
          icon: Icons.Calendar,
          href: '/calendar',
        },
        {
          name: t('operations.projects'),
          icon: Icons.Scroll,
          href: '/projects',
        },
      ],
    },
    {
      name: t('enums.index'),
      icon: Icons.CheckCircle,
      children: [
        {
          name: t('enums.task-types'),
          icon: Icons.CheckCircle,
          href: '/enums/task-types',
        },
        {
          name: t('enums.positions'),
          icon: Icons.Briefcase,
          href: '/enums/positions',
        },
      ],
    },
    {
      name: t('cashflow.index'),
      icon: Icons.Money,
      children: [
        {
          name: t('cashflow.categories'),
          icon: Icons.Coin,
          href: '/cashflow/opex/categories',
        },
        {
          name: t('cashflow.payments'),
          icon: Icons.Coin,
          href: '/cashflow/payments',
        },
      ],
      href: '',
    },
    {
      name: t('reports.index'),
      icon: Icons.FileText,
      children: [
        {
          name: t('reports.cashflow'),
          icon: Icons.Money,
          href: '/reports/cashflow',
        },
      ],
      href: '',
    },
  ];
  return (
    <div key="1" className="flex w-full flex-col bg-gray-950 shadow-lg p-6">
      <div className="flex h-16 items-center justify-center">
        <Link className="flex items-center gap-2" href="#">
          <Image
            src="/svg/iota-1c.png"
            alt="Logo"
            width="150"
            height="50"
          />
        </Link>
      </div>
      <nav className="flex-1 overflow-y-auto py-4">
        <ul className="flex flex-col gap-2 h-full">
          {links.map((link) => (
            <SidebarItem key={link.name} {...link} />
          ))}
          <div className="flex-grow"/>
          <SidebarLink
            className="mt-4"
            name={t('logout')}
            icon={Icons.SignOut}
            href="/logout"
            active={false}
            isExpanded={false}
          />
        </ul>
      </nav>
    </div>
  );
}
