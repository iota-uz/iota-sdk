'use client';
import {ApolloClient, ApolloProvider, HttpLink, InMemoryCache, split} from "@apollo/client";
import {GraphQLWsLink} from "@apollo/client/link/subscriptions";
import {createClient} from "graphql-ws";
import {getMainDefinition} from "@apollo/client/utilities";
import React from "react";

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
            definition.kind === 'OperationDefinition'
            && definition.operation === 'subscription'
        );
    },
    wsLink,
    httpLink,
);

const client = new ApolloClient({
    link: splitLink,
    cache: new InMemoryCache(),
});

export function ApolloWrapper({children}: React.PropsWithChildren) {
    return (
        <ApolloProvider client={client}>
            {children}
        </ApolloProvider>
    );
}