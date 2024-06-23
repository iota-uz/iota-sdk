'use client';

import 'katex/dist/katex.min.css';
import * as Icons from '@phosphor-icons/react';
import {useMutation, useQuery, useSubscription} from '@apollo/client';
import React, {
    createContext, useContext, useEffect, useState,
} from 'react';
import Link from 'next/link';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {Button} from '@/components/ui/button';
import {Card} from '@/components/ui/card';
import {Dialogue, Message} from '@/src/__generated__/graphql';
import ModelDropdown from '@/components/composite/model-dropdown';
import {cn} from '@/lib/utils';
import Spinner from '@/components/icons/spinner';
import {Avatar, AvatarFallback, AvatarImage} from '@/components/ui/avatar';
import {
    ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger,
} from '@/components/ui/context-menu';
import ChatInput from '@/components/composite/chat-input';
import LlmOutput from '@/components/composite/llm-output';
import {
    DELETE_DIALOGUE,
    GET_DIALOGUE,
    GET_DIALOGUES,
    NEW_DIALOGUE,
    ON_DIALOGUE_CREATED,
    ON_DIALOGUE_UPDATED,
    REPLY_TO_DIALOGUE,
} from '@/app/(app)/bi-chat/graphql';
import Image from "next/image";

type ChatContextType = {
    newDialogue: (message: string) => Promise<void>;
    deleteDialogue: (id: string) => Promise<void>;
}

const ChatContext = createContext<ChatContextType>({
    newDialogue: async (msg) => {
    },
    deleteDialogue: async (id) => {
    },
});

function EmptyChatHistory() {
    return (
        <div className="flex flex-col items-center justify-center flex-grow gap-4 px-4">
            <Image
                width="120"
                height="93"
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
    );
}

function ChatItem({dialogue, onDelete}: { dialogue: Dialogue, onDelete: (id: string) => void }) {
    const [editMode, setEditMode] = useState(false);

    const params = useSearchParams();
    const active = params.get('chat') === dialogue.id.toString();
    const {deleteDialogue} = useContext(ChatContext);

    if (editMode) {
        return (
            <li className="flex justify-between rounded-lg">
                <div className="flex gap-1 w-full">
                    <input
                        type="text"
                        className="w-full p-2"
                        value={dialogue.label}
                        onChange={(e) => {
                            dialogue.label = e.target.value;
                        }}
                        onBlur={() => {
                            setEditMode(false);
                        }}
                    />
                    <button
                        className="bg-primary-100 hover:bg-primary-200 transition-all duration-150 rounded-lg p-2"
                        onClick={() => {
                            setEditMode(false);
                        }}
                    >
                        <Icons.Check size="18"/>
                    </button>
                    <button
                        className="bg-red-100 hover:bg-red-200 transition-all duration-150 rounded-lg p-2"
                        onClick={() => {
                            setEditMode(false);
                        }}
                    >
                        <Icons.X size="18"/>
                    </button>
                </div>
            </li>
        );
    }

    return (
        <li className={cn('flex justify-between w-full', active ? 'text-primary-800' : '')}>
            <ContextMenu>
                <ContextMenuTrigger className="w-full">
                    <Link
                        href={`/bi-chat?chat=${dialogue.id}`}
                        className={
                            cn('block rounded-lg p-2 w-full hover:bg-primary-100', active ? 'bg-primary-200' : '')
                        }
                    >
                        {dialogue.label}
                    </Link>
                </ContextMenuTrigger>
                <ContextMenuContent>
                    <ContextMenuItem onClick={() => {
                        setEditMode(true);
                    }}
                    >
                        <button className="flex gap-3">
                            <Icons.PencilSimple size="18"/>
                            Переименовать
                        </button>
                    </ContextMenuItem>
                    <ContextMenuItem onClick={() => {
                        deleteDialogue(dialogue.id);
                        onDelete(dialogue.id);
                    }}
                    >
                        <button className="flex gap-3">
                            <Icons.TrashSimple size="18"/>
                            Удалить
                        </button>
                    </ContextMenuItem>
                </ContextMenuContent>
            </ContextMenu>
        </li>
    );
}

function ChatHistory() {
    const {
        data, loading, error, refetch,
    } = useQuery(GET_DIALOGUES, {});
    const router = useRouter();
    const [dialogues, setDialogues] = useState<Dialogue[]>(data?.dialogues.data || []);

    useEffect(() => {
        setDialogues(data?.dialogues.data || []);
    }, [data]);

    useSubscription(ON_DIALOGUE_CREATED, {
        onData: (opts) => {
            const {dialogueCreated} = opts.data.data;
            setDialogues([...dialogues, dialogueCreated]);
            router.push(`/bi-chat?chat=${dialogueCreated.id}`);
        },
    });

    function handleDeleteDialogue(id: string) {
        setDialogues(dialogues.filter((dialogue) => dialogue.id !== id));
    }

    if (error) {
        return (
            <div className="flex flex-col justify-center flex-grow p-4 text-center">
                <h1 className="text-xl">Ошибка при загрузке чатов</h1>
                <p className="text-sm">
                    {error.message}
                </p>
                <div className="flex justify-center mt-4">
                    <Button onClick={() => refetch()}>
                        Повторить
                    </Button>
                </div>
            </div>
        );
    }

    if (loading) {
        return (
            <div className="flex flex-grow justify-center items-center">
                <div className="scale-150">
                    <Spinner/>
                </div>
            </div>
        );
    }

    if (!dialogues.length) {
        return (
            <EmptyChatHistory/>
        );
    }
    const children = dialogues.map((dialogue) => (
        <ChatItem
            key={dialogue.id}
            dialogue={dialogue}
            onDelete={handleDeleteDialogue}
        />
    ));
    return (
        <div className="flex flex-col flex-grow border-y border-primary-200 py-4 px-5">
            <span className="text-gray-500">Недавнее</span>
            <ul>
                {children}
            </ul>
        </div>
    );
}

function ChatSideBar() {
    const [model, setModel] = useState('gpt-3.5');
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();

    function onSetModel(model: string) {
        setModel(model);
        const params = new URLSearchParams(searchParams);
        params.set('model', model);
        router.push(`${pathname}?${params.toString()}`);
    }

    return (
        <div className="flex flex-col border-r border-primary-200 min-w-80 max-w-80">
            <div className="flex justify-center py-4">
                <Link href="/bi-chat">
                    <Button className="flex gap-2" variant="secondary">
                        <Icons.PlusCircle weight="bold"/>
                        Новый диалог
                    </Button>
                </Link>
            </div>
            <ChatHistory/>
            <div>
                <ModelDropdown value={model} onSelect={onSetModel}/>
            </div>
        </div>
    );
}

export type ExamplePromptType = {
    top: string;
    bottom: string;
}

function ExamplePrompt({prompt, onClick}: { prompt: ExamplePromptType, onClick: (msg: string) => void }) {
    return (
        <Card
            className="flex items-center gap-4 px-2.5 py-4 cursor-pointer hover:bg-gray-200"
            onClick={() => onClick(`${prompt.top} ${prompt.bottom}`)}
        >
            <div>
                <p className="text-gray-950">
                    {prompt.top}
                </p>
                <p className="text-gray-500">
                    {prompt.bottom}
                </p>
            </div>
            <div className="border-2 border-gray-400 p-0.5 rounded-md">
                <Icons.ArrowUp className="text-gray-400" size="14" weight="bold"/>
            </div>
        </Card>
    );
}

function AiAvatar() {
    return (
        <div className="flex justify-center items-center bg-gray-50 rounded-full">
            <Image
                width="32"
                height="32"
                src="/svg/iota-1c-only.png"
                alt="AI Avatar"
            />
        </div>
    );
}

function UserAvatar() {
    return (
        <div className="flex justify-center items-center bg-gray-50 rounded-full">
            <Avatar className="h-8 w-8">
                <AvatarImage alt="@shadcn" src="/placeholder-avatar.jpg"/>
                <AvatarFallback className="bg-primary-600 text-white">JP</AvatarFallback>
            </Avatar>
        </div>
    );
}

function ChatMessage({role, content, ...props}: Message & React.ComponentProps<'div'>) {
    return (
        <div {...props}>
            <div className="flex items-center gap-2">
                {role === 'user' ? <UserAvatar/> : <AiAvatar/>}
                <p className="text-gray-950 font-semibold">
                    {role === 'user' ? 'Вы' : 'IotaGPT'}
                </p>
            </div>
            {
                role === 'tool' ? ('Делаю запрос в систему') : (
                    <LlmOutput
                        className="mt-2"
                        content={content}
                    />
                )
            }
            <div className="flex gap-2 mt-2 text-gray-600">
                <button>
                    <Icons.SpeakerHigh size="20" weight="bold"/>
                </button>
                <button>
                    <Icons.Copy size="20"/>
                </button>
                <button>
                    <Icons.ArrowsClockwise size="20"/>
                </button>
            </div>
        </div>
    );
}

function NewDialogue() {
    const {newDialogue} = useContext(ChatContext);

    const examplePrompts = [
        {top: 'Помоги спланировать', bottom: 'стратегию погашения задолженостей'},
        {top: 'Помоги спланировать', bottom: 'стратегию погашения задолженостей'},
        {top: 'Помоги спланировать', bottom: 'стратегию погашения задолженостей'},
        {top: 'Помоги спланировать', bottom: 'стратегию погашения задолженостей'},
    ];
    return (
        <div className="grid grid-cols-2 gap-2.5">
            {
                examplePrompts.map((prompt, index) => (
                    <ExamplePrompt key={index} prompt={prompt} onClick={newDialogue}/>
                ))
            }
        </div>
    );
}

function DialogueComponent({dialogueId}: { dialogueId: number }) {
    const {
        data, loading, error, refetch,
    } = useQuery(GET_DIALOGUE, {
        variables: {id: dialogueId},
    });
    const [dialogue, setDialogue] = useState<Dialogue | null>(null);
    useEffect(() => {
        setDialogue(data?.dialogue || null);
    });

    useSubscription(ON_DIALOGUE_UPDATED, {
        onData: (opts) => {
            setDialogue(opts.data.data.dialogueUpdated);
        },
    });

    if (error) {
        return (
            <div className="flex flex-col justify-center flex-grow p-4 text-center">
                <h1 className="text-xl">Ошибка при загрузке сообщений</h1>
                <p className="text-sm">
                    {error.message}
                </p>
                <div className="flex justify-center mt-4">
                    <Button onClick={refetch}>
                        Повторить
                    </Button>
                </div>
            </div>
        );
    }

    if (loading) {
        return (
            <div className="flex flex-grow justify-center items-center">
                <div className="scale-150">
                    <Spinner/>
                </div>
            </div>
        );
    }

    if (!dialogue) {
        return <EmptyChatHistory/>;
    }

    return <Messages dialogue={dialogue}/>;
}

function Messages({dialogue}: { dialogue: Dialogue }) {
    const messages = [...(dialogue.messages || [])].reverse();

    return (
        <div className="flex flex-col-reverse flex-grow overflow-y-auto gap-4 px-20 pt-20 w-full">
            {
                messages.map((message, index) => (
                    <ChatMessage
                        key={index}
                        {...message}
                    />
                ))
            }
        </div>
    );
}

function Chat() {
    const params = useSearchParams();
    const dialogueId = params.get('chat') ? parseInt(params.get('chat') as string) : null;
    const [dialogue, setDialogue] = useState<Dialogue | null>(null);

    useSubscription(ON_DIALOGUE_CREATED, {
        onData: (opts) => {
            const {dialogueCreated} = opts.data.data;
            setDialogue(dialogueCreated);
        },
    });

    useSubscription(ON_DIALOGUE_UPDATED, {
        onData: (opts) => {
            setDialogue(opts.data.data.dialogueUpdated);
        },
    });

    if (dialogue) {
        return <Messages dialogue={dialogue}/>;
    }

    if (dialogueId) {
        return <DialogueComponent dialogueId={dialogueId}/>;
    }

    return <NewDialogue/>;
}

export default function Component() {
    const [createNewDialogue] = useMutation(NEW_DIALOGUE);
    const [replyToDialogue] = useMutation(REPLY_TO_DIALOGUE);
    const [_deleteDialogue] = useMutation(DELETE_DIALOGUE);
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();
    const dialogueId = searchParams.get('chat') ? parseInt(searchParams.get('chat') as string) : null;

    async function newDialogue(message: string) {
        const input = {
            message,
            model: searchParams.get('model') || 'gpt-3.5',
        };
        if (dialogueId) {
            await replyToDialogue({variables: {id: dialogueId, input}});
            return;
        }
        const response = await createNewDialogue({variables: {input}});
        const {id} = response.data.newDialogue;
        const params = new URLSearchParams(searchParams);
        params.set('chat', id);
        router.push(`/${pathname}?${params.toString()}`);
    }

    async function deleteDialogue(id: string) {
        await _deleteDialogue({
            variables: {id},
        });
        router.push('/bi-chat');
    }

    return (
        <div className="container mx-auto px-8 py-8">
            <div>
                <h1 className="text-2xl font-medium">
                    BI-чат
                </h1>
            </div>
            <ChatContext.Provider value={{newDialogue, deleteDialogue}}>
                <Card className="h-chat flex">
                    <ChatSideBar/>
                    <div className="flex flex-col flex-grow items-center justify-end gap-4">
                        <Chat/>
                        <div className="flex justify-center w-full py-5">
                            <ChatInput onSubmit={newDialogue}/>
                        </div>
                    </div>
                </Card>
            </ChatContext.Provider>
        </div>
    );
}
