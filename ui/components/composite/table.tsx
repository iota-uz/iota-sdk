import {DateTime} from 'luxon';
import {IMask} from "react-imask";
import {CSSProperties} from "react";
import {useRouter} from "next/navigation";
import {cn} from "@/lib/utils";
import Spinner from "@/components/icons/spinner";

export type Column<T> = {
    label: string; // label displayed in column
    key: string; // required column for :key
    width?: number; // column width between 0 and 100
    field?: (item: T) => any; // if field function is provided it will use it to fill column content. You can also pass raw html here
    subtitle?: (item: T) => any; // same as field, but is displayed below the field content
    dateFormat?: string; // example: "calendar" or "MMM DD". More info about formatting in moment docs
    duration?: boolean;
    sortable?: boolean; // if true the column will become sortable
    classes?: string; // css classes to apply
    enums?: { [index: string]: string }; // if provided it will use the enum value to display the label
    decimalPoints?: number; // if provided it will round the number to the specified decimal points
    mask?: string; // if provided it will format the number using mask, ex.: +1 (###) ###-####
    style?: {
        overwrite?: boolean; // if true, overwrite default styles
        value: CSSProperties; // example: {width: "100px"}
    };
}

export type SortBy<T> = { [key in keyof T]?: 'asc' | 'desc' };

export type Props<T extends object> = {
    primaryKey: keyof T;
    columns: Column<T>[];
    data: T[];
    sortBy?: SortBy<T>;
    loading?: boolean;
    onSort?: (sortBy: SortBy<T>) => void;
    selected?: (keyof T)[];
    selector?: (t: any) => any;
}

function getValue(item: any, column: Column<any>) {
    if (column.field) {
        return column.enums ? column.enums[column.field(item)] : column.field(item);
    }
    if (column.enums) {
        return column.enums[item[column.key]];
    }
    if (column.mask) {
        return new IMask.MaskedPattern({
            mask: column.mask,
        }).resolve(item[column.key]);
    }
    if (column.decimalPoints !== undefined) {
        return (item[column.key] || 0).toFixed(column.decimalPoints);
    }
    if (column.duration) {
        return DateTime.fromISO(item[column.key]).toRelative();
    }
    if (column.dateFormat) {
        return formatDate(item[column.key], column.dateFormat);
    }
    return item[column.key];
}

function formatDate(date: any, format: string): string {
    if (!date) {
        return '-';
    }
    if (format === 'calendar') {
        return DateTime.fromISO(date).toRelativeCalendar() || '-';
    }
    return DateTime.fromISO(date).toFormat(format);
}

function columnStyle<T extends object>(column: Column<T>): CSSProperties {
    const defaultStyle: CSSProperties = {
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        maxWidth: '150px'
    };
    if (column.width) {
        defaultStyle.width = `${column.width}%`;
        return defaultStyle;
    }
    if (!column.style)
        return defaultStyle;

    if (column.style.overwrite) {
        return column.style.value;
    }
    return {...defaultStyle, ...column.style.value};
}

function TableCell<T extends object>({item, column}: { item: T, column: Column<T> }) {
    const router = useRouter();
    const classes = cn(
        "px-2.5 py-3 whitespace-nowrap text-sm bg-primary-50 border-primary-200 text-gray-500 dark:text-gray-400",
        column.classes
    );
    return (
        <td key={column.key}
            className={classes}
            style={columnStyle(column)}>
            {getValue(item, column)}
            {column.subtitle && <div className="text-gray-400 text-sm">
                {column.subtitle(item)}
            </div>}
        </td>
    )
}

function TableBody<T extends object>({loading, data, columns}: Props<T>) {
    const columnCount = columns.length;
    if (loading) {
        return (
            <tbody>
            <tr>
                <td colSpan={columnCount} className="py-4">
                    <div className="flex justify-center">
                        <Spinner/>
                    </div>
                </td>
            </tr>
            </tbody>
        );
    }
    if (!data.length) {
        return (
            <tbody>
            <tr>
                <td colSpan={columnCount} className="text-center py-4 rounded-lg">Пока ничего</td>
            </tr>
            </tbody>
        );
    }

    return (
        <tbody>
        {
            data.map((item, index) => (
                <tr>
                    {columns.map((column) => <TableCell key={column.key} item={item} column={column}/>)}
                </tr>
            ))
        }
        </tbody>
    );
}

function Chevron({direction}: { direction: 'asc' | 'desc' }) {
    return (
        <span className="ml-1">
            {direction === 'asc' ? <span>&darr;</span> : <span>&uarr;</span>}
        </span>
    );
}

function TableHeaderCell<T extends object>({column, sortBy, onSort}: {
    column: Column<T>,
    sortBy?: SortBy<T>,
    onSort?: (sortBy: SortBy<T>) => void
}) {
    const classes = cn(
        "border-primary-200 text-center text-sm font-medium text-gray-950 dark:text-gray-400",
        column.label ? "p-4" : ""
    );
    if (!sortBy || !column.sortable || !onSort) {
        return (
            <th className={classes}>
                {column.label}
            </th>
        );
    }
    const name = column.key as keyof T;
    const direction = sortBy[name];
    const toggleDirection = direction === 'asc' ? 'desc' : 'asc';
    return (
        <th className={classes}>
            <a
                href="#"
                onClick={(e) => {
                    e.preventDefault();
                    onSort({[name]: toggleDirection} as SortBy<T>);
                }}
            >
                {column.label}
            </a>
            {direction && <Chevron direction={direction}/>}
        </th>
    );
}

function TableHeader<T extends object>({columns, sortBy, onSort}: Props<T>) {
    return (
        <thead className="mb-4">
        <tr className="bg-primary-100 dark:bg-gray-800 rounded-lg">
            {
                columns.map((column) => (
                        <TableHeaderCell
                            key={column.key}
                            column={column}
                            sortBy={sortBy}
                            onSort={onSort}/>
                    )
                )
            }
        </tr>
        </thead>
    );
}

export default function BaseTable<T extends object>(props: Props<T>) {
    if (props.columns.some((column) => column.sortable && !props.onSort)) {
        throw new Error('You need to provide onSort callback for sortable columns');
    }
    return (
        <div className="overflow-x-auto relative">
            <table className="min-w-full table-auto rounded-table">
                <TableHeader {...props}/>
                <TableBody {...props}/>
            </table>
        </div>
    );
}

BaseTable.defaultProps = {
    loading: false,
    bulkActions: false,
    selected: [],
    selector: (t: any) => t
}