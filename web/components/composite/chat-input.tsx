import React, {useState} from "react";
import {Input} from "@/components/ui/input";
import * as Icons from "@phosphor-icons/react";

export type Props = {
    onSubmit: (message: string) => void;
}

export default function ChatInput({onSubmit}: Props) {
    const [message, setMessage] = useState('');
    const onClick = () => {
        onSubmit(message);
        setMessage('');
    };
    return (
        <div className="relative w-[75%]">
            <Input placeholder="Поиск"
                   value={message}
                   onKeyUp={(e) => {
                       if (e.key !== 'Enter') {
                           return;
                       }
                       onClick();
                   }}
                   onChange={(e) => setMessage(e.target.value)}
                   className="px-4 h-12 border-primary-200 placeholder-gray-400 bg-primary-50 rounded-xl"
            />
            <button className="bg-gray-800 p-0.5 max-w-fit rounded-md absolute right-4 top-4" onClick={onClick}>
                <Icons.ArrowUp className="text-white" size="14" weight="bold"/>
            </button>
        </div>
    )
}
