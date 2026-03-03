import * as React from "react";
import { Check, ChevronsUpDown, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

interface MultiSelectOption {
  label: string;
  value: string;
}

interface MultiSelectProps {
  options: MultiSelectOption[];
  selected: string[];
  onChange: (selected: string[]) => void;
  placeholder?: string;
  className?: string;
}

export function MultiSelect({
  options,
  selected,
  onChange,
  placeholder = "Select items...",
  className,
}: MultiSelectProps) {
  const [open, setOpen] = React.useState(false);

  const handleToggle = (value: string) => {
    if (selected.includes(value)) {
      onChange(selected.filter((item) => item !== value));
    } else {
      onChange([...selected, value]);
    }
  };

  const handleClear = (e: React.MouseEvent) => {
    e.stopPropagation();
    onChange([]);
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn(
            "min-w-[180px] h-10 justify-between",
            selected.length > 0 ? "text-foreground font-medium" : "text-muted-foreground font-normal",
            className
          )}
        >
          <div className="flex gap-1 items-center overflow-hidden">
            {selected.length === 0 && <span className="truncate">{placeholder}</span>}
            {selected.length > 0 && (
              <span className="text-sm">
                {selected.length} selected
              </span>
            )}
          </div>
          <div className="flex items-center gap-1">
            {selected.length > 0 && (
              <button
                type="button"
                onClick={handleClear}
                className="rounded-sm opacity-70 ring-offset-background hover:opacity-100"
              >
                <X className="h-3 w-3" />
              </button>
            )}
            <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
          </div>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[250px] p-0" align="start">
        <div className="max-h-64 overflow-y-auto p-1">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => handleToggle(option.value)}
              className={cn(
                "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground",
                selected.includes(option.value) && "bg-accent"
              )}
            >
              <div className={cn(
                "flex h-4 w-4 items-center justify-center rounded-sm border border-primary",
                selected.includes(option.value) ? "bg-primary text-primary-foreground" : "bg-background"
              )}>
                {selected.includes(option.value) && <Check className="h-3 w-3" />}
              </div>
              <span className="flex-1 text-left">{option.label}</span>
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
