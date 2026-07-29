#Förslag för hårdvaruprogrammeraren

##Purpose
Att se hur vi kan berika sec-språket så att det är lätt för programmeraren att jobba med hårdvara.

## 1. Register-arrayer med offset (För likadana block)

I nästan alla MCU:er upprepas hårdvarublock. Du har sällan bara en timer eller en UART; du har UART0, UART1, UART2. 
Att definiera adresser manuellt för varje enskilt register skapar enormt mycket boilerplate.
Ergonomisk lösning: Tillåt definitioner av register-block (strukturer av register) som kan instansieras på en basadress, eller arrayer med fasta avstånd (stride).

Exempel på syntax:
```sec
type UART block {
    Data: UART_DataRegister,       // Offset 0x00
    Status: UART_StatusRegister,   // Offset 0x04
    Baud: bit[32],                 // Offset 0x08 (råtyp)
}

@address(0x40011000)
let mut UART1: UART

@address(0x40011400)
let mut UART2: UART
```

### Motivation för "Block" (Varför det behövs för STM32/ESP32)
Eftersom du programmerar STM32 känner du till periferienheter (peripherals). En STM32F4 har t.ex. upp till 8 stycken UART/USART-enheter. 
Varje enskild UART består av en exakt likadan sekvens av register: SR (Status), DR (Data), BRR (Baud rate), CR1, CR2, CR3.Om språket inte har ett sätt att gruppera 
register till en återanvändbar struktur (ett block), tvingas programmeraren göra något av följande (vilket skapar boilerplate eller sänker ergonomin):
a. Definiera UART1_SR, UART1_DR, UART2_SR, UART2_DR manuellt med unika @address. 
   (Extremt mycket boilerplate).

b. Använda råa minnesoffsetter manuellt i koden: let sr = @address(UART1_BASE + 0x00). 
   (Tappar typsäkerheten).
   
Den generella lösningen:Eftersom språket ska konkurrera med Go och Zig, har du troligtvis redan vanliga datastrukturer (struct). 
Du behöver inte en ny kategori som heter block. Du kan helt enkelt tillåta att en vanlig struct kan placeras på en fast adress, och att dess fält kan vara dina register-typer:

```sec
type UartPeripheral struct {
    Status: StatusRegister,   // Ligger på offset +0x00
    Data: DataRegister,       // Ligger på offset +0x04 (om Status tar 32 bitar)
    Baud: BaudRegister,       // Ligger på offset +0x08
}

// Nu kan vi mappa hela hårdvarublocket på en gång!
@address(0x40011000)
let mut UART1: UartPeripheral

@address(0x40011400)
let mut UART2: UartPeripheral
```

Varför hårdvarufantaster kommer älska detta: Det efterliknar exakt hur CMSIS (C-biblioteket för ARM) är uppbyggt med struct-pekare (USART1->SR), fast med ditt språks inbyggda säkerhet och noll behov av osäkra typkonverteringar ((USART_TypeDef *)). För en webbprogrammerare (PHP/Go) känns det bara som en vanlig struct.

## 2. Explicit hantering av Write-Only (Clear-on-Write / Toggle-on-Write)

Inbäddad hårdvara har konstiga registerbeteenden. 
Ofta rensar man ett avbrott (interrupt flag) genom att skriva en 1 till den biten (kallas W1C - Write 1 to Clear), eller så kraschar systemet om man försöker 
läsa från ett Write-Only-register.Problemet med mut: mut antyder Read-Modify-Write (läs nuvarande värde, ändra en bit, skriv tillbaka). Om biten är Write-Only, 
blir läsningen skräp och du skriver tillbaka fel värde.Ergonomisk lösning: Introducera @writeonly eller ett sätt att markera destruktiva skrivningar där språket vet att det inte ska läsa registret först.

Exempel på syntax:
```sec
type InterruptRegister register[32] {
    TimerOverflown: bit writeonly,      // Genererar en ren Store (W1C) utan föregående läsning
    HardwareError: bit readonly,        // Kan aldrig skrivas till av mjukvaran
    InterruptFlags: bit[8] clearbyread, // 
    Enabled: bit,                       // Läs- och skrivbar om variabeln deklareras 'mut'
    _: bit[21],
}
```

## 3. Bit-Enums (Slipp magiska nummer)

När programmerare läser datablad ser de ofta tabeller som: 00 = 115200 baud, 01 = 9600 baud, 10 = ExtClock. I vanliga språk tvingas man använda konstater 
eller magiska siffror.Ergonomisk lösning: Låt en bit-kontrollerad typ ta emot en hårdvaru-mappad enum. Kompilatorn (via LLVM) validerar att värdena får 
plats i de tilldelade bitarna.

Exempel på syntax:
```sec
enum ClockSource: bit[2] {
    Internal = 0b00,
    External = 0b01,
    Bypass   = 0b10,
}

type ClockConfig register[32] {
    Source: ClockSource, // Tar exakt 2 bitar i anspråk!
    Enabled: bit,
    _: bit[29],
}
```

**Varför detta är magiskt: **
Om utvecklaren skriver config.
Source = 5, kastar din kompilator ett fel direkt eftersom 5 (0b101) kräver 3 bitar. 
Ingen boilerplate, maximal säkerhet.

## 4. Noll-boilerplate Avbrottshantering (Interrupts)

I C/C++ måste man ofta pricka exakta, magiska funktionsnamn (t.ex. `void USART1_IRQHandler(void)`) som länkaren (linker) förväntar sig i avbrottsvektorn. 
Missar du ett tecken blir det inget felmeddelande, men koden körs aldrig.

### Lösning
Eftersom sec använder `@` för kompilatorinstruktioner, kan en funktion dekoreras direkt för att tala om för LLVM att binda den till hårdvarans avbrottsvektor. 
Kompilatorn och LLVM hanterar då automatiskt "prologue/epilogue" (sparar och återställer CPU-register på stacken).


Exempel på syntax:
```sec
@interrupt(Vector.USART1)
fn handle_uart() void {
    // Språket och LLVM sköter "prologue/epilogue" (sparar register på stacken) automatiskt.
    let status = UART1.Status
}
```

## 5. "Safe Casts" av rådata till Registerlayouter

När man bygger inbäddade system läser man ofta data från en SPI-buss, DMA eller ett flashminne som en rå ström av bytes eller ord (`bit[8]` eller `bit[32]`). 
Detsamma gäller när en backend-utvecklare parsar nätverkspaket från en socket.

### Lösning
Typkonvertering sker enhetligt via `Typ(data)`. Detta tillåter utvecklaren att direkt gjuta om rådata till en validerad och ergonomisk register/bit-layout.

```sec
let raw_data: bit[32] = read_spi_word()

// Sömlös och säker omvandling till en strukturerad layout
let packet = SystemRegister(raw_data) 
```