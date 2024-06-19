'use client';

import {Settings} from "luxon";
import {Inter as FontSans} from "next/font/google"
import {createClient} from 'graphql-ws';
import * as Icons from "@phosphor-icons/react";
import {ApolloClient, ApolloProvider, createHttpLink, HttpLink, InMemoryCache, split} from '@apollo/client';
import {GraphQLWsLink} from '@apollo/client/link/subscriptions';
import {getMainDefinition} from '@apollo/client/utilities';

import "./globals.css"
import {cn} from "@/lib/utils"
import {Sidebar} from "@/components/sidebar";
import Navbar from "@/components/navbar";


const fontSans = FontSans({
    subsets: ["latin"],
    variable: "--font-sans",
})


const httpLink = new HttpLink({
    uri: 'http://localhost:3200/query',
    credentials: 'include',
});

const wsLink = new GraphQLWsLink(createClient({
    url: 'ws://localhost:3200/query',
}));

const splitLink = split(
    ({query}) => {
        const definition = getMainDefinition(query);
        return (
            definition.kind === 'OperationDefinition' &&
            definition.operation === 'subscription'
        );
    },
    wsLink,
    httpLink,
);

const client = new ApolloClient({
    link: splitLink,
    cache: new InMemoryCache(),
});

export default function RootLayoutWithApollo({children}: { children: React.ReactNode }) {
    Settings.defaultLocale = "ru";
    return <ApolloProvider client={client}>
        <RootLayout>
            {children}
        </RootLayout>
    </ApolloProvider>
}

const links = [
    {
        name: 'Панель управления',
        icon: Icons.Gauge,
        href: '/'
    },
    {
        name: 'Пользователи',
        icon: Icons.Users,
        href: '/users',
    },
    {
        name: 'BI-чат',
        icon: Icons.ChatCircle,
        href: '/bi-chat',
    },
    {
        name: 'Операционка',
        icon: Icons.Pulse,
        children: [
            {
                name: 'Сотрудники',
                icon: Icons.Users,
                href: '/employees'
            },
            {
                name: 'Настройки',
                icon: Icons.Gear,
                href: '/settings',
            },
            {
                name: 'Календарь',
                icon: Icons.Calendar,
                href: '/calendar',
            },
            {
                name: 'Проекты',
                icon: Icons.Scroll,
                href: '/projects',
            },
        ]
    },
    {
        name: 'Справочники',
        icon: Icons.CheckCircle,
        children: [
            {
                name: 'Типы задач',
                icon: Icons.CheckCircle,
                href: '/enums/task-types',
            },
            {
                name: 'Должности',
                icon: Icons.Briefcase,
                href: '/enums/positions',
            },
        ]
    },
    {
        name: 'ДДС',
        icon: Icons.Money,
        children: [
            {
                name: 'ОПЕКС',
                icon: Icons.Coin,
                href: '/cashflow/opex/categories',
            },
            {
                name: 'Платежи',
                icon: Icons.Coin,
                href: '/cashflow/payments',
            },
        ],
        href: ""
    },
    {
        name: 'Отчеты',
        icon: Icons.FileText,
        children: [
            {
                name: 'Отчет ДДС',
                icon: Icons.Money,
                href: '/reports/cashflow',
            },
        ],
        href: ""
    },
];

export function RootLayout({children}: { children: React.ReactNode }) {
    return (
        <html lang="en" suppressHydrationWarning>
        <head>
            <title>
                IOTA ERP
            </title>
        </head>
        <body
            className={cn(
                "min-h-screen bg-background font-sans antialiased",
                fontSans.variable
            )}
        >
        <div key="1" className="grid min-h-screen w-full grid-cols-[280px_1fr]">
            <Sidebar links={links}/>
            <div>
                <Navbar/>
                {children}
            </div>
        </div>
        </body>
        </html>
    )
}
