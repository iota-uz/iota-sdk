'use client';
import {Button} from "@/components/ui/button"
import * as Icons from "@phosphor-icons/react"
import {SortBy} from "@/components/composite/table";
import {gql, useQuery} from "@apollo/client";
import React, {useState} from "react";
import Link from "next/link";
import {Card} from "@/components/ui/card";
import {Dialogue, User} from "@/src/__generated__/graphql";
import {Input} from "@/components/ui/input";
import ModelDropdown from "@/components/composite/modelDropdown";
import {cn} from "@/lib/utils";
import {Spinner} from "@phosphor-icons/react";

const GET_DIALOGUES = gql`
    query GetDialogues {
        dialogues(limit: 100, offset: 0) {
            data {
                id
                label
            }
        }
    }
`;

function EmptyChatHistory() {
    return (
        <div className="flex flex-col items-center gap-4">
            <img
                width="120"
                src="/svg/empty-chat-history.svg"
                alt="Empty chat history illustration"
            />
            <p className="text-center text-gray-950">
                Нет истории чатов
            </p>
            <p className="text-center text-gray-500">
                Нажмите кнопку "Новый диалог", чтобы начать новый чат
            </p>
        </div>
    )
}

function ChatHistory({dialogues}: { dialogues: Dialogue[] }) {
    const children = dialogues.map((dialogue) => (
        <li className="bg-gray-50 hover:bg-primary-200 rounded-lg">
            <Link href={`/bi-chat/${dialogue.id}`} className="w-full block p-2">
                {dialogue.label}
            </Link>
        </li>
    ));
    return (
        <div>
            Недавнее
            <ul>
                {children}
            </ul>
        </div>
    )
}

function ChatSideBar({dialogues, loading}: { dialogues: Dialogue[], loading: boolean }) {
    if (loading) {
        return <Spinner/>
    }
    return (
        <div className="w-96 border-r border-primary-200 flex flex-col">
            <div className="flex justify-center py-4">
                <Button className="flex gap-2" variant="secondary">
                    <Icons.PlusCircle weight="bold"/>
                    Новый диалог
                </Button>
            </div>
            <div
                className={cn("px-5 border-y border-primary-200 flex flex-col flex-grow", dialogues.length ? "py-4" : "justify-center")}>
                {dialogues.length > 0 ? (<ChatHistory dialogues={dialogues}/>) : (<EmptyChatHistory/>)}
            </div>
            <div>
                <ModelDropdown/>
            </div>
        </div>
    )
}

export default function Component() {
    const [perPage, setPerPage] = useState(10);
    const [page, setPage] = useState(1);
    const [sortBy, setSortBy] = useState<SortBy<User>>({
        createdAt: 'desc'
    });
    const {data, loading: areDialoguesLoading, error, refetch} = useQuery(GET_DIALOGUES, {
        variables: {
            limit: perPage,
            offset: (page - 1) * perPage,
            sortBy: Object.keys(sortBy).map((key) => `${key} ${sortBy[key as keyof User]}`)
        }
    });

    const {data: dialogues} = data?.dialogues || {total: 0, data: []};
    if (error) {
        return (
            <div className="container mx-auto px-8 py-8">
                <h1 className="text-2xl">Ошибка при загрузке пользователей</h1>
                <p className="text-lg">
                    {error.message}
                </p>
                <code>
                    {error.stack}
                </code>
                <div className="flex justify-center mt-4">
                    <Button onClick={() => refetch()}>
                        Повторить
                    </Button>
                </div>
            </div>
        );
    }

    return (
        <div className="container mx-auto px-8 py-8">
            <div>
                <h1 className="text-2xl font-medium">
                    BI-чат
                </h1>
            </div>
            <Card className="h-chat flex">
                <ChatSideBar dialogues={dialogues} loading={areDialoguesLoading}/>
                <div className="flex flex-col flex-grow justify-end items-center">
                    <div className="flex-grow">
                    </div>
                    <div className="grid grid-cols-2 gap-2.5">
                        <Card className="flex items-center gap-4 px-2.5 py-4">
                            <div>
                                <p className="text-gray-950">
                                    Помоги спланировать
                                </p>
                                <p className="text-gray-500">
                                    стратегию погашения задолженостей
                                </p>
                            </div>
                            <div className="border-2 border-gray-400 p-0.5 rounded-md">
                                <Icons.ArrowUp className="text-gray-400" size="14" weight="bold"/>
                            </div>
                        </Card>
                        <Card className="flex items-center gap-4 px-2.5 py-4">
                            <div>
                                <p className="text-gray-950">
                                    Помоги спланировать
                                </p>
                                <p className="text-gray-500">
                                    стратегию погашения задолженостей
                                </p>
                            </div>
                            <div className="border-2 border-gray-400 p-0.5 rounded-md">
                                <Icons.ArrowUp className="text-gray-400" size="14" weight="bold"/>
                            </div>
                        </Card>
                        <Card className="flex items-center gap-4 px-2.5 py-4">
                            <div>
                                <p className="text-gray-950">
                                    Помоги спланировать
                                </p>
                                <p className="text-gray-500">
                                    стратегию погашения задолженостей
                                </p>
                            </div>
                            <div className="border-2 border-gray-400 p-0.5 rounded-md">
                                <Icons.ArrowUp className="text-gray-400" size="14" weight="bold"/>
                            </div>
                        </Card>
                        <Card className="flex items-center gap-4 px-2.5 py-4">
                            <div>
                                <p className="text-gray-950">
                                    Помоги спланировать
                                </p>
                                <p className="text-gray-500">
                                    стратегию погашения задолженостей
                                </p>
                            </div>
                            <div className="border-2 border-gray-400 p-0.5 rounded-md">
                                <Icons.ArrowUp className="text-gray-400" size="14" weight="bold"/>
                            </div>
                        </Card>
                    </div>
                    <div className="flex justify-center w-full py-5">
                        <div className="relative w-[75%]">
                            <Input placeholder="Поиск"
                                   className="px-4 h-12 border-primary-200 placeholder-gray-400 bg-primary-50 rounded-xl"/>
                            <div className="bg-gray-800 p-0.5 max-w-fit rounded-md absolute right-4 top-4">
                                <Icons.ArrowUp className="text-white" size="14" weight="bold"/>
                            </div>
                        </div>
                    </div>
                </div>
            </Card>
        </div>
    )
}