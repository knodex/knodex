import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";

interface TableLoadingSkeletonProps {
  /** Column definitions for the table header */
  columns: Array<{
    header: string;
    width?: string;
    hideOnMobile?: boolean;
  }>;
  /** Number of rows to display */
  rows?: number;
  /** Whether to show double-line cell content (name + description pattern) */
  showDoubleLines?: boolean;
}

/**
 * Loading skeleton component for tables
 * AC-SHARED-03: Loading skeleton for tables
 */
export function TableLoadingSkeleton({
  columns,
  rows = 5,
  showDoubleLines = false,
}: TableLoadingSkeletonProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          {columns.map((col, index) => (
            <TableHead
              key={index}
              className={col.hideOnMobile ? "hidden md:table-cell" : ""}
              style={col.width ? { width: col.width } : undefined}
            >
              {col.header}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {Array.from({ length: rows }).map((_, rowIndex) => (
          <TableRow key={rowIndex}>
            {columns.map((col, colIndex) => (
              <TableCell
                key={colIndex}
                className={col.hideOnMobile ? "hidden md:table-cell" : ""}
              >
                {colIndex === 0 && showDoubleLines ? (
                  <div className="space-y-1">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-3 w-48" />
                  </div>
                ) : (
                  <Skeleton
                    className={
                      colIndex === 0 ? "h-4 w-32" : "h-4 w-20"
                    }
                  />
                )}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export default TableLoadingSkeleton;
