import React from 'react';
import {ChevronDown} from 'lucide-react';
import {DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger} from '@/components/ui/dropdown-menu';

export type Props = {
    value: string;
    onSelect: (model: string) => void;
} & React.ComponentProps<typeof DropdownMenu>;

export default function ModelDropdown(props: Props) {
    const models = [
        {name: 'GPT-3.5', value: 'gpt-3.5'},
        {name: 'GPT-4o', value: 'gpt-4o'},
    ];
    const selected = models.find(model => model.value === props.value);

    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <button className="flex justify-center gap-2 p-8 w-full">
                    {selected?.name}
                    <ChevronDown/>
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-64" align="center" side="bottom">
                {
                    models.map(model => (
                        <DropdownMenuItem key={model.value}
                                          onClick={() => props.onSelect(model.value)}
                                          className="cursor-pointer">
                            {model.name}
                        </DropdownMenuItem>
                    ))
                }
            </DropdownMenuContent>
        </DropdownMenu>
    )
}