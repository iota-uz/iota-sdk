import {Settings} from 'luxon';
import {Inter as FontSans} from 'next/font/google';
import {NextIntlClientProvider} from 'next-intl';
import {getLocale, getMessages} from 'next-intl/server';
import React, {Suspense} from 'react';

import './globals.css';
import {cn} from '@/lib/utils';
import {Sidebar} from '@/components/sidebar';
import Navbar from '@/components/navbar';
import {ApolloWrapper} from "@/app/apollo-wrapper";

const fontSans = FontSans({
    subsets: ['latin'],
    variable: '--font-sans',
});

export default async function RootLayoutWithApollo({children}: { children: React.ReactNode }) {
    Settings.defaultLocale = 'ru';
    const locale = await getLocale();
    const messages = await getMessages();
    return (
        <html lang={locale} suppressHydrationWarning>
        <head>
            <title>
                IOTA ERP
            </title>
        </head>
        <body
            className={cn(
                'min-h-screen bg-background font-sans antialiased',
                fontSans.variable,
            )}
        >
        <ApolloWrapper>
            <Suspense fallback="Loading...">
                <NextIntlClientProvider messages={messages}>
                    {children}
                </NextIntlClientProvider>
            </Suspense>
        </ApolloWrapper>
        </body>
        </html>
    );
}
