import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select";

export type Props = {
    perPage: number;
    setPerPage: (perPage: number) => void;
    options?: number[];
}

export default function PerPageSelect(props: Props) {
    return (
        <Select className="w-24" defaultValue="10" onValueChange={(v) => props.setPerPage(parseInt(v))}>
            <SelectTrigger>
                <SelectValue placeholder="10"/>
            </SelectTrigger>
            <SelectContent>
                {props.options?.map(option => (
                    <SelectItem key={option} onSelect={() => props.setPerPage(option)} value={option.toString()}>
                        {option}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
}