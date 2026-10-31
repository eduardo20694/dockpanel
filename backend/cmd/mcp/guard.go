package main

import (
	"errors"

	"dockerpilot/internal/envx"
)

func ensureWritable() error {
	if envx.ReadOnly() {
		return errors.New("DOCKERPILOT_READONLY está ativo: esta ferramenta altera estado e foi bloqueada. Use uma ferramenta de leitura (list_*, diagnose_container, what_broke, security_audit, cost_insights) ou desligue o modo somente leitura")
	}
	return nil
}
